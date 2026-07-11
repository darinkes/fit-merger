package fit

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/filedef"
)

// Inspect returns ordered, human-readable key/value details specific to a FIT
// file: its file_id header, message counts, and the summary figures the device
// actually stored (as opposed to the ones fitmerge recomputes). Invalid/unset
// fields are omitted.
func Inspect(path string) ([][2]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fit, err := decoder.New(f).Decode()
	if err != nil {
		return nil, fmt.Errorf("decode fit %q: %w", path, err)
	}
	a := filedef.NewActivity(fit.Messages...)

	var kv kvList
	kv.add("File type", a.FileId.Type.String())
	kv.add("Manufacturer", a.FileId.Manufacturer.String())
	if a.FileId.Product != basetype.Uint16Invalid {
		kv.add("Product", strconv.Itoa(int(a.FileId.Product)))
	}
	if a.FileId.ProductName != "" {
		kv.add("Product name", a.FileId.ProductName)
	}
	if a.FileId.SerialNumber != basetype.Uint32Invalid {
		kv.add("Serial number", strconv.FormatUint(uint64(a.FileId.SerialNumber), 10))
	}
	if !a.FileId.TimeCreated.IsZero() {
		kv.add("Time created", a.FileId.TimeCreated.UTC().Format(time.RFC3339))
	}

	kv.add("Records", strconv.Itoa(len(a.Records)))
	kv.add("Sessions", strconv.Itoa(len(a.Sessions)))
	kv.add("Laps", strconv.Itoa(len(a.Laps)))
	if len(a.Events) > 0 {
		kv.add("Events", strconv.Itoa(len(a.Events)))
	}
	if len(a.DeviceInfos) > 0 {
		kv.add("Device infos", strconv.Itoa(len(a.DeviceInfos)))
	}

	if a.Activity != nil {
		kv.add("Activity type", a.Activity.Type.String())
	}

	// Stored session summary — what a device wrote and other tools read directly.
	if len(a.Sessions) > 0 {
		s := a.Sessions[0]
		kv.add("Stored sport", s.Sport.String())
		kv.addFloat("Stored distance", s.TotalDistanceScaled(), "%.1f m")
		if s.TotalAscent != basetype.Uint16Invalid {
			kv.add("Stored ascent", fmt.Sprintf("%d m", s.TotalAscent))
		}
		if s.TotalDescent != basetype.Uint16Invalid {
			kv.add("Stored descent", fmt.Sprintf("%d m", s.TotalDescent))
		}
		kv.addDuration("Stored moving time", s.TotalTimerTimeScaled())
		kv.addDuration("Stored elapsed time", s.TotalElapsedTimeScaled())
		kv.addFloat("Stored avg speed", s.AvgSpeedScaled(), "%.2f m/s")
		kv.addFloat("Stored max speed", s.MaxSpeedScaled(), "%.2f m/s")
		if s.AvgHeartRate != basetype.Uint8Invalid {
			kv.add("Stored avg HR", fmt.Sprintf("%d bpm", s.AvgHeartRate))
		}
		if s.MaxHeartRate != basetype.Uint8Invalid {
			kv.add("Stored max HR", fmt.Sprintf("%d bpm", s.MaxHeartRate))
		}
		if s.TotalCalories != basetype.Uint16Invalid {
			kv.add("Stored calories", fmt.Sprintf("%d kcal", s.TotalCalories))
		}
	}
	return kv, nil
}

// kvList accumulates ordered key/value detail pairs.
type kvList [][2]string

func (l *kvList) add(k, v string) { *l = append(*l, [2]string{k, v}) }

func (l *kvList) addFloat(k string, v float64, format string) {
	if math.IsNaN(v) {
		return
	}
	l.add(k, fmt.Sprintf(format, v))
}

func (l *kvList) addDuration(k string, seconds float64) {
	if math.IsNaN(seconds) {
		return
	}
	d := time.Duration(seconds * float64(time.Second)).Round(time.Second)
	l.add(k, d.String())
}

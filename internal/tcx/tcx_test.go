package tcx_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
	"github.com/darinkes/fitmerge/internal/tcx"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2" xmlns:ns3="http://www.garmin.com/xmlschemas/ActivityExtension/v2">
  <Activities>
    <Activity Sport="Biking">
      <Id>2024-01-01T10:00:00Z</Id>
      <Lap StartTime="2024-01-01T10:00:00Z">
        <TotalTimeSeconds>10</TotalTimeSeconds>
        <DistanceMeters>50</DistanceMeters>
        <Track>
          <Trackpoint>
            <Time>2024-01-01T10:00:00Z</Time>
            <Position><LatitudeDegrees>47.0</LatitudeDegrees><LongitudeDegrees>8.0</LongitudeDegrees></Position>
            <AltitudeMeters>500</AltitudeMeters>
            <DistanceMeters>0</DistanceMeters>
            <HeartRateBpm><Value>120</Value></HeartRateBpm>
            <Cadence>85</Cadence>
            <Extensions><ns3:TPX><ns3:Speed>5.5</ns3:Speed><ns3:Watts>200</ns3:Watts></ns3:TPX></Extensions>
          </Trackpoint>
          <Trackpoint>
            <Time>2024-01-01T10:00:10Z</Time>
            <Position><LatitudeDegrees>47.001</LatitudeDegrees><LongitudeDegrees>8.001</LongitudeDegrees></Position>
            <AltitudeMeters>505</AltitudeMeters>
            <DistanceMeters>50</DistanceMeters>
            <HeartRateBpm><Value>130</Value></HeartRateBpm>
          </Trackpoint>
        </Track>
      </Lap>
    </Activity>
  </Activities>
</TrainingCenterDatabase>`

func TestReadParsesTrackpointsAndExtensions(t *testing.T) {
	act, err := tcx.Read(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if act.Sport != "cycling" {
		t.Errorf("sport = %q, want cycling", act.Sport)
	}
	if len(act.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(act.Records))
	}

	r0 := act.Records[0]
	if r0.Lat == nil || *r0.Lat != 47.0 || r0.Lon == nil || *r0.Lon != 8.0 {
		t.Errorf("r0 position = %v,%v", r0.Lat, r0.Lon)
	}
	if r0.Altitude == nil || *r0.Altitude != 500 {
		t.Errorf("r0 altitude = %v, want 500", r0.Altitude)
	}
	if r0.HR == nil || *r0.HR != 120 {
		t.Errorf("r0 hr = %v, want 120", r0.HR)
	}
	if r0.Cadence == nil || *r0.Cadence != 85 {
		t.Errorf("r0 cadence = %v, want 85", r0.Cadence)
	}
	if r0.Speed == nil || *r0.Speed != 5.5 {
		t.Errorf("r0 speed = %v, want 5.5", r0.Speed)
	}
	if r0.Power == nil || *r0.Power != 200 {
		t.Errorf("r0 power = %v, want 200", r0.Power)
	}
	if want := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC); !r0.Time.Equal(want) {
		t.Errorf("r0 time = %v, want %v", r0.Time, want)
	}

	// Second point has no cadence/power/speed — those must stay absent, not 0.
	r1 := act.Records[1]
	if r1.Power != nil || r1.Cadence != nil || r1.Speed != nil {
		t.Errorf("r1 optional fields should be nil: power=%v cadence=%v speed=%v", r1.Power, r1.Cadence, r1.Speed)
	}
	if r1.HR == nil || *r1.HR != 130 {
		t.Errorf("r1 hr = %v, want 130", r1.HR)
	}
}

func TestRoundTripPreservesRecords(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	hr := func(v uint8) *uint8 { return &v }
	pw := func(v uint16) *uint16 { return &v }
	base := time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC)
	act := model.Activity{
		Sport: "running",
		Records: []model.Record{
			{Time: base, Lat: f(48.1), Lon: f(11.5), Altitude: f(515), Distance: f(0), HR: hr(110), Power: pw(150)},
			{Time: base.Add(time.Second), Lat: f(48.2), Lon: f(11.6), Altitude: f(520), Distance: f(140), HR: hr(115)},
		},
	}
	sum := model.Summary{StartTime: base, EndTime: base.Add(time.Second), TotalDistance: 140, TotalElapsed: time.Second, MaxHR: 115, AvgHR: 112}

	var buf bytes.Buffer
	if err := tcx.Write(&buf, act, sum); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `Sport="Running"`) {
		t.Errorf("expected Sport=\"Running\" in output")
	}

	back, err := tcx.Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if back.Sport != "running" {
		t.Errorf("sport round-trip = %q, want running", back.Sport)
	}
	if len(back.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(back.Records))
	}
	r0 := back.Records[0]
	if r0.Lat == nil || *r0.Lat != 48.1 || r0.Altitude == nil || *r0.Altitude != 515 {
		t.Errorf("r0 position/altitude not preserved: %v %v", r0.Lat, r0.Altitude)
	}
	if r0.HR == nil || *r0.HR != 110 || r0.Power == nil || *r0.Power != 150 {
		t.Errorf("r0 hr/power not preserved: %v %v", r0.HR, r0.Power)
	}
	if !r0.Time.Equal(base) {
		t.Errorf("r0 time = %v, want %v", r0.Time, base)
	}
	if back.Records[1].Power != nil {
		t.Errorf("r1 power should stay absent, got %v", back.Records[1].Power)
	}
}

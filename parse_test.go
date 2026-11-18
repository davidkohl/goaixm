package goaixm_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
)

const basicFile = "testdata/basic_message.xml"

func TestParseBasicMessage(t *testing.T) {
	msg := parseFile(t, basicFile, goaixm.Options{})
	if msg.Kind != "AIXMBasicMessage" || msg.GMLID != "BasicMessage_01" {
		t.Errorf("message kind %q id %q", msg.Kind, msg.GMLID)
	}
	if n := len(msg.Features()); n != 3 {
		t.Errorf("expected 3 features, got %d", n)
	}

	as := goaixm.FeaturesOf[*aixm.Airspace](msg)
	if len(as) != 1 {
		t.Fatalf("expected 1 airspace, got %d", len(as))
	}
	if got := as[0].UUID(); got != "e34e0e14-0001-0000-0000-000000000000" {
		t.Errorf("airspace identifier = %q", got)
	}
	ts := as[0].TimeSlice[0]
	if ts.Interpretation != aixm.Baseline {
		t.Errorf("interpretation = %q", ts.Interpretation)
	}
	if ts.Designator.String() != "EGTT" {
		t.Errorf("designator = %q", ts.Designator.String())
	}

	ad := goaixm.FeaturesOf[*aixm.AirportHeliport](msg)
	if len(ad) != 1 || ad[0].UUID() != "e34e0e14-0002-0000-0000-000000000000" ||
		ad[0].TimeSlice[0].Designator.String() != "EGLL" {
		t.Errorf("airport = %+v", ad)
	}

	nav := goaixm.FeaturesOf[*aixm.Navaid](msg)
	if len(nav) != 1 || nav[0].TimeSlice[0].Type.String() != "NDB" {
		t.Errorf("navaid = %+v", nav)
	}
	if got := len(msg.ByKind("Navaid")); got != 1 {
		t.Errorf("ByKind(Navaid) = %d", got)
	}
}

func TestResolve(t *testing.T) {
	msg := parseFile(t, basicFile, goaixm.Options{})
	const id = "e34e0e14-0001-0000-0000-000000000000"
	for _, tc := range []struct {
		href string
		want bool
	}{
		{"urn:uuid:" + id, true},
		{"#" + id, true},
		{id, true},
		{"#xpointer(//*[@gml:identifier='" + id + "'])", true},
		{"urn:uuid:00000000-dead-beef-0000-000000000000", false},
		{"", false},
	} {
		f, ok := msg.Resolve(tc.href)
		if ok != tc.want {
			t.Errorf("Resolve(%q) ok=%v, want %v", tc.href, ok, tc.want)
			continue
		}
		if ok {
			if _, isAirspace := f.(*aixm.Airspace); !isAirspace {
				t.Errorf("Resolve(%q) = %T", tc.href, f)
			}
		}
	}
}

func TestResolveRouteSegmentStart(t *testing.T) {
	msg := parseFile(t, "testdata/xlink_message.xml", goaixm.Options{})
	if n := len(msg.Features()); n != 2 {
		t.Fatalf("expected 2 features, got %d", n)
	}
	segs := goaixm.FeaturesOf[*aixm.RouteSegment](msg)
	if len(segs) != 1 {
		t.Fatalf("expected 1 route segment, got %d", len(segs))
	}
	start := segs[0].TimeSlice[0].Start
	if start == nil || start.PointChoiceFixDesignatedPoint == nil {
		t.Fatalf("start = %+v", start)
	}
	f, ok := msg.Resolve(start.PointChoiceFixDesignatedPoint.Href)
	if !ok {
		t.Fatalf("start point %q not resolved", start.PointChoiceFixDesignatedPoint.Href)
	}
	dp, ok := f.(*aixm.DesignatedPoint)
	if !ok || dp.TimeSlice[0].Designator.String() != "ALPHA" {
		t.Errorf("resolved %T %+v", f, f)
	}
}

func TestParseADRMessage(t *testing.T) {
	msg := parseFile(t, "testdata/adr_message.xml", goaixm.Options{})
	if msg.GMLID != "ADRMessage_01" {
		t.Errorf("message id = %q", msg.GMLID)
	}
	if len(msg.Features()) == 0 {
		t.Error("expected at least one feature")
	}
}

func TestOptionsMaxBytes(t *testing.T) {
	data, err := os.ReadFile(basicFile)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	msg, err := goaixm.Parse(ctx, bytes.NewReader(data), goaixm.Options{MaxBytes: int64(len(data))})
	if err != nil {
		t.Fatalf("within limit: %v", err)
	}
	if len(msg.Features()) != 3 {
		t.Errorf("expected 3 features, got %d", len(msg.Features()))
	}
	_, err = goaixm.Parse(ctx, bytes.NewReader(data), goaixm.Options{MaxBytes: 10})
	if !errors.Is(err, goaixm.ErrTooLarge) {
		t.Errorf("over limit: err = %v, want ErrTooLarge", err)
	}
}

func TestOptionsStrict(t *testing.T) {
	data, err := os.ReadFile(basicFile)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	empty := goaixm.NewRegistry()

	msg, err := goaixm.Parse(ctx, bytes.NewReader(data), goaixm.Options{Registry: empty})
	if err != nil {
		t.Fatalf("lenient: %v", err)
	}
	if len(msg.Features()) != 0 || len(msg.Warnings()) != 3 {
		t.Errorf("lenient: features=%d warnings=%d", len(msg.Features()), len(msg.Warnings()))
	}
	if !errors.Is(msg.Warnings()[0], goaixm.ErrUnknownFeature) {
		t.Errorf("warning does not wrap ErrUnknownFeature: %v", msg.Warnings()[0])
	}

	_, err = goaixm.Parse(ctx, bytes.NewReader(data), goaixm.Options{Registry: empty, Strict: true})
	if !errors.Is(err, goaixm.ErrUnknownFeature) {
		t.Errorf("strict: err = %v, want ErrUnknownFeature", err)
	}
}

func TestOptionsOnWarningAborts(t *testing.T) {
	data, err := os.ReadFile(basicFile)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stop")
	_, err = goaixm.Parse(context.Background(), bytes.NewReader(data), goaixm.Options{
		Registry:  goaixm.NewRegistry(),
		OnWarning: func(goaixm.Warning) error { return stop },
	})
	if !errors.Is(err, stop) {
		t.Errorf("err = %v, want stop", err)
	}
}

func TestParseCancelled(t *testing.T) {
	data, err := os.ReadFile(basicFile)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := goaixm.Parse(ctx, bytes.NewReader(data), goaixm.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestParseRejectsNonXML(t *testing.T) {
	if _, err := goaixm.Parse(context.Background(), bytes.NewReader(nil), goaixm.Options{}); err == nil {
		t.Error("empty input parsed without error")
	}
}

func TestHrefID(t *testing.T) {
	for in, want := range map[string]string{
		"urn:uuid:abc": "abc",
		"#abc":         "abc",
		"  #abc ":      "abc",
		"http://x/y":   "",
		"#xpointer(//*[@gml:identifier='1b54b2d6-a5ff-4e57-94c2-f4047a381c64'])": "1b54b2d6-a5ff-4e57-94c2-f4047a381c64",
		"1b54b2d6-a5ff-4e57-94c2-f4047a381c64":                                   "1b54b2d6-a5ff-4e57-94c2-f4047a381c64",
	} {
		if got := goaixm.HrefID(in); got != want {
			t.Errorf("HrefID(%q) = %q, want %q", in, got, want)
		}
	}
}

// donlonFile is the EUROCONTROL DONLON reference data set (git submodule).
const donlonFile = "testdata/donlon/EA_AIP_DS_FULL_20170701.xml"

// requireDonlon skips the test when the DONLON submodule is not checked out.
func requireDonlon(t testing.TB) {
	t.Helper()
	if _, err := os.Stat(donlonFile); err != nil {
		t.Skip("DONLON reference data missing; run: git submodule update --init")
	}
}

func parseFile(t testing.TB, path string, opts goaixm.Options) *goaixm.Message {
	t.Helper()
	msg, err := goaixm.ParseFile(context.Background(), path, opts)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return msg
}

func parseDonlon(t testing.TB, opts goaixm.Options) *goaixm.Message {
	t.Helper()
	requireDonlon(t)
	return parseFile(t, donlonFile, opts)
}

func TestStreamMatchesParse(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	kinds := map[string]int{}
	var designators []string
	err := goaixm.StreamFile(context.Background(), donlonFile, goaixm.Options{}, func(m goaixm.Member) error {
		kinds[m.Kind]++
		if a, ok := m.Feature.(*aixm.Airspace); ok {
			designators = append(designators, a.TimeSlice[0].Designator.String())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for k, n := range kinds {
		total += n
		if got := len(msg.ByKind(k)); got != n {
			t.Errorf("%s: streamed %d, parsed %d", k, n, got)
		}
	}
	if total != len(msg.Features()) {
		t.Errorf("streamed %d members, parsed %d", total, len(msg.Features()))
	}
	if len(designators) == 0 || designators[0] == "" {
		t.Errorf("airspace designators not decoded: %v", designators)
	}
}

func TestStreamCancellation(t *testing.T) {
	requireDonlon(t)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err := goaixm.StreamFile(ctx, donlonFile, goaixm.Options{}, func(goaixm.Member) error {
		if n++; n == 3 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStreamHandlerError(t *testing.T) {
	requireDonlon(t)
	stop := errors.New("stop now")
	err := goaixm.StreamFile(context.Background(), donlonFile, goaixm.Options{}, func(goaixm.Member) error {
		return stop
	})
	if !errors.Is(err, stop) {
		t.Errorf("err = %v, want handler error", err)
	}
}

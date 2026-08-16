package fonts

import "testing"

func TestParseUnicodeRanges(t *testing.T) {
	cases := []struct {
		name    string
		spec    string
		want    []RuneRange
		wantErr bool
	}{
		{
			name: "single codepoint",
			spec: "U+0301",
			want: []RuneRange{{0x0301, 0x0301}},
		},
		{
			name: "explicit range",
			spec: "U+0102-0103",
			want: []RuneRange{{0x0102, 0x0103}},
		},
		{
			name: "vietnamese block",
			spec: "U+1EA0-1EF9",
			want: []RuneRange{{0x1EA0, 0x1EF9}},
		},
		{
			name: "two digit wildcard",
			spec: "U+00??",
			want: []RuneRange{{0x0000, 0x00FF}},
		},
		{
			name: "single digit wildcard",
			spec: "U+4??",
			want: []RuneRange{{0x0400, 0x04FF}},
		},
		{
			name: "all wildcard",
			spec: "U+??",
			want: []RuneRange{{0x0000, 0x00FF}},
		},
		{
			name: "comma list with spaces and lowercase prefix",
			spec: "u+0000-00ff, U+0131 ,U+0152-0153,U+02BB-02BC",
			want: []RuneRange{
				{0x0000, 0x00FF},
				{0x0131, 0x0131},
				{0x0152, 0x0153},
				{0x02BB, 0x02BC},
			},
		},
		{
			name: "google vietnamese subset",
			spec: googleVietnameseRange,
			want: []RuneRange{
				{0x0102, 0x0103}, {0x0110, 0x0111}, {0x0128, 0x0129}, {0x0168, 0x0169},
				{0x01A0, 0x01A1}, {0x01AF, 0x01B0}, {0x0300, 0x0301}, {0x0303, 0x0304},
				{0x0308, 0x0309}, {0x0323, 0x0323}, {0x0329, 0x0329}, {0x1EA0, 0x1EF9},
				{0x20AB, 0x20AB},
			},
		},
		{name: "empty", spec: "   ", wantErr: true},
		{name: "missing prefix", spec: "0301", wantErr: true},
		{name: "reversed range", spec: "U+0103-0102", wantErr: true},
		{name: "wildcard mixed with range", spec: "U+00??-01FF", wantErr: true},
		{name: "wildcard not trailing", spec: "U+?0FF", wantErr: true},
		{name: "not hex", spec: "U+zzzz", wantErr: true},
		{name: "too many digits", spec: "U+1234567", wantErr: true},
		{name: "beyond unicode max", spec: "U+110000", wantErr: true},
		{name: "empty element in list", spec: "U+0301,,U+0303", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUnicodeRanges(tc.spec)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseUnicodeRanges(%q) = %v, want an error", tc.spec, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUnicodeRanges(%q): %v", tc.spec, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseUnicodeRanges(%q) = %v, want %v", tc.spec, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("ParseUnicodeRanges(%q)[%d] = %v, want %v", tc.spec, i, got[i], tc.want[i])
				}
			}
		})
	}
}

const (
	googleLatinRange = "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC," +
		"U+0304,U+0308,U+0329,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193," +
		"U+2212,U+2215,U+FEFF,U+FFFD"

	googleVietnameseRange = "U+0102-0103,U+0110-0111,U+0128-0129,U+0168-0169,U+01A0-01A1," +
		"U+01AF-01B0,U+0300-0301,U+0303-0304,U+0308-0309,U+0323,U+0329,U+1EA0-1EF9,U+20AB"

	// "Tuyen bo quyet dinh cua Dai hoi dong", fully precomposed.
	vietnameseSample = "Tuyên bố quyết định của Đại hội đồng"
)

func TestRangesCover_VietnameseSampleNeedsVietnameseSubset(t *testing.T) {
	latin, err := ParseUnicodeRanges(googleLatinRange)
	if err != nil {
		t.Fatalf("parse latin: %v", err)
	}
	missing := RangesCover(latin, vietnameseSample)
	if len(missing) == 0 {
		t.Fatal("the latin subset alone reports full coverage of a Vietnamese sample; the range test is not testing anything")
	}
	for _, r := range missing {
		if r < 0x0100 {
			t.Fatalf("U+%04X reported missing from the latin range U+0000-00FF", r)
		}
	}

	withVietnamese, err := ParseUnicodeRanges(googleLatinRange + "," + googleVietnameseRange)
	if err != nil {
		t.Fatalf("parse latin+vietnamese: %v", err)
	}
	if missing := RangesCover(withVietnamese, vietnameseSample); len(missing) != 0 {
		t.Fatalf("latin+vietnamese leaves %+q missing, want full coverage", string(missing))
	}
}

func TestRangesCover_FirstAppearanceOrderAndDeduped(t *testing.T) {
	ascii, err := ParseUnicodeRanges("U+00??")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sample := oHorn + " " + uHorn + " " + oHorn + " " + aCircumflexGrave
	if got, want := string(RangesCover(ascii, sample)), oHorn+uHorn+aCircumflexGrave; got != want {
		t.Fatalf("RangesCover = %+q, want %+q", got, want)
	}
	if got := RangesCover(ascii, "plain latin"); len(got) != 0 {
		t.Fatalf("RangesCover = %+q, want none", string(got))
	}
	if got := RangesCover(nil, "x"); string(got) != "x" {
		t.Fatalf("RangesCover(nil, %q) = %q, want %q", "x", string(got), "x")
	}
}

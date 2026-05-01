package base62

import "testing"

func TestIntToString(t *testing.T) {
	tests := []struct {
		name string
		seq  uint64
		want string
	}{
		// TODO: Add test cases.
		{name: "0", seq: 0, want: "0"},
		{name: "1", seq: 1, want: "1"},
		{name: "62", seq: 62, want: "10"},
		{name: "6342", seq: 6347, want: "1En"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntToString(tt.seq); got != tt.want {
				t.Errorf("IntToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToInt(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		wantSeq uint64
	}{
		// TODO: Add test cases.
		{name: "0", s: "0", wantSeq: 0},
		{name: "10", s: "10", wantSeq: 62},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotSeq := StringToInt(tt.s); gotSeq != tt.wantSeq {
				t.Errorf("StringToInt() = %v, want %v", gotSeq, tt.wantSeq)
			}
		})
	}
}

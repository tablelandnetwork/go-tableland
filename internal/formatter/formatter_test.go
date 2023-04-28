package formatter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/gateway"
)

var rawJSON = []byte("{\"city\":\"dallas\"}")

var input = &gateway.TableData{
	Columns: []gateway.Column{
		{Name: "name"},
		{Name: "age"},
		{Name: "location"},
	},
	Rows: [][]*gateway.ColumnValue{
		{gateway.OtherColValue("bob"), gateway.OtherColValue(40), gateway.JSONColValue(rawJSON)},
		{gateway.OtherColValue("jane"), gateway.OtherColValue(30), gateway.JSONColValue(rawJSON)},
	},
}

var inputExtractable = &gateway.TableData{
	Columns: []gateway.Column{
		{Name: "name"},
	},
	Rows: [][]*gateway.ColumnValue{
		{gateway.OtherColValue("bob")},
		{gateway.OtherColValue("jane")},
	},
}

var inputExtractable2 = &gateway.TableData{
	Columns: []gateway.Column{
		{Name: "location"},
	},
	Rows: [][]*gateway.ColumnValue{
		{gateway.JSONColValue(rawJSON)},
		{gateway.JSONColValue(rawJSON)},
	},
}

func TestFormat(t *testing.T) {
	type args struct {
		userRows *gateway.TableData
		output   Output
		unwrap   bool
		extract  bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "table",
			args: args{userRows: input, output: Table},
			want: "{\"columns\":[{\"name\":\"name\"},{\"name\":\"age\"},{\"name\":\"location\"}],\"rows\":[[\"bob\",40,{\"city\":\"dallas\"}],[\"jane\",30,{\"city\":\"dallas\"}]]}", //nolint
		},
		{
			name: "objects",
			args: args{userRows: input, output: Objects},
			want: "[{\"name\":\"bob\",\"age\":40,\"location\":{\"city\":\"dallas\"}},{\"name\":\"jane\",\"age\":30,\"location\":{\"city\":\"dallas\"}}]", // nolint
		},
		{
			name: "objects, extract",
			args: args{userRows: inputExtractable, output: Objects, extract: true},
			want: "[\"bob\",\"jane\"]",
		},
		{
			name: "objects, extract nested json",
			args: args{userRows: inputExtractable2, output: Objects, extract: true},
			want: "[{\"city\":\"dallas\"},{\"city\":\"dallas\"}]",
		},
		{
			name:    "objects, extract error",
			args:    args{userRows: input, output: Objects, extract: true},
			wantErr: true,
		},
		{
			name: "objects, unwrap",
			args: args{userRows: input, output: Objects, unwrap: true},
			want: "{\"name\":\"bob\",\"age\":40,\"location\":{\"city\":\"dallas\"}}\n{\"name\":\"jane\",\"age\":30,\"location\":{\"city\":\"dallas\"}}\n", // nolint
		},
		{
			name: "objects, extract, unwrap",
			args: args{userRows: inputExtractable, output: Objects, extract: true, unwrap: true},
			want: "\"bob\"\n\"jane\"",
		},
		{
			name: "objects, extract, unwrap nested json",
			args: args{userRows: inputExtractable2, output: Objects, extract: true, unwrap: true},
			want: "{\"city\":\"dallas\"}\n{\"city\":\"dallas\"}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Format(
				tt.args.userRows,
				WithOutput(tt.args.output),
				WithUnwrap(tt.args.unwrap),
				WithExtract(tt.args.extract),
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if tt.args.unwrap {
				wantStrings := parseJSONLString(tt.want)
				gotStrings := parseJSONLString(string(got))
				require.Equal(t, len(wantStrings), len(gotStrings))
				for i, wantString := range wantStrings {
					require.JSONEq(t, wantString, gotStrings[i])
				}
			} else {
				require.JSONEq(t, tt.want, string(got))
			}
		})
	}
}

func parseJSONLString(val string) []string {
	s := strings.TrimRight(val, "\n")
	return strings.Split(s, "\n")
}

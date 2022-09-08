package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
)

var input = &tableland.UserRows{
	Columns: []tableland.UserColumn{
		{Name: "name"},
		{Name: "age"},
	},
	Rows: [][]*tableland.ColValue{
		{tableland.OtherColValue("bob"), tableland.OtherColValue(40)},
		{tableland.OtherColValue("jane"), tableland.OtherColValue(30)},
	},
}

var inputExtractable = &tableland.UserRows{
	Columns: []tableland.UserColumn{
		{Name: "name"},
	},
	Rows: [][]*tableland.ColValue{
		{tableland.OtherColValue("bob")},
		{tableland.OtherColValue("jane")},
	},
}

func TestFormat(t *testing.T) {
	type args struct {
		userRows *tableland.UserRows
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
			want: "{\"columns\":[{\"name\":\"name\"},{\"name\":\"age\"}],\"rows\":[[\"bob\",40],[\"jane\",30]]}",
		},
		{
			name: "objects",
			args: args{userRows: input, output: Objects},
			want: "[{\"name\":\"bob\",\"age\":40},{\"name\":\"jane\",\"age\":30}]",
		},
		{
			name: "objects, extract",
			args: args{userRows: inputExtractable, output: Objects, extract: true},
			want: "[\"bob\",\"jane\"]",
		},
		{
			name:    "objects, extract error",
			args:    args{userRows: input, output: Objects, extract: true},
			wantErr: true,
		},
		{
			name: "objects, unwrap",
			args: args{userRows: input, output: Objects, unwrap: true},
			want: "{\"name\":\"bob\",\"age\":40}\n{\"name\":\"jane\",\"age\":30}\n",
		},
		{
			name: "objects, extract, unwrap",
			args: args{userRows: inputExtractable, output: Objects, extract: true, unwrap: true},
			want: "\"bob\"\n\"jane\"",
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
				gotStrings, err := parseJSONLBytes(got)
				require.NoError(t, err)
				require.Equal(t, len(wantStrings), len(gotStrings))
				for i, wantString := range wantStrings {
					require.JSONEq(t, wantString, gotStrings[i])
				}
			} else {
				b, err := json.Marshal(got)
				require.NoError(t, err)
				require.JSONEq(t, tt.want, string(b))
			}
		})
	}
}

func parseJSONLBytes(val interface{}) ([]string, error) {
	b, ok := val.([]byte)
	if !ok {
		return nil, fmt.Errorf("error converting value to []byte")
	}
	s := string(b)
	return parseJSONLString(s), nil
}

func parseJSONLString(val string) []string {
	s := strings.TrimRight(val, "\n")
	return strings.Split(s, "\n")
}

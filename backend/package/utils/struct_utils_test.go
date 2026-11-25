package utils_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type product struct {
	ID    int      `json:"id" yaml:"id"`
	Name  string   `json:"name" yaml:"name"`
	Price float64  `json:"price" yaml:"price"`
	Tags  []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func TestFromStringToObjectJSON_Struct(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    product
		wantErr bool
	}{
		{
			name:  "valid object",
			input: `{"id":1,"name":"Shoe","price":49.99,"tags":["sale","new"]}`,
			want:  product{ID: 1, Name: "Shoe", Price: 49.99, Tags: []string{"sale", "new"}},
		},
		{
			name:  "missing fields keep zero values",
			input: `{"name":"Hat"}`,
			want:  product{Name: "Hat"},
		},
		{
			name:  "unknown fields are ignored",
			input: `{"id":2,"extra":"ignored"}`,
			want:  product{ID: 2},
		},
		{
			name:  "empty object",
			input: `{}`,
			want:  product{},
		},
		{
			name:    "malformed json",
			input:   `{"id":1,`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "wrong field type",
			input:   `{"id":"not-a-number"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.FromStringToObjectJSON[product](tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFromStringToObjectJSON_OtherTypes(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		got, err := utils.FromStringToObjectJSON[map[string]any](`{"a":1,"b":"x","c":true}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := map[string]any{"a": float64(1), "b": "x", "c": true}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("slice", func(t *testing.T) {
		got, err := utils.FromStringToObjectJSON[[]int](`[1,2,3]`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []int{1, 2, 3}) {
			t.Errorf("got %v, want [1 2 3]", got)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		got, err := utils.FromStringToObjectJSON[*product](`{"id":7}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.ID != 7 {
			t.Errorf("got %+v, want &{ID:7}", got)
		}
	})

	t.Run("null into pointer gives nil", func(t *testing.T) {
		got, err := utils.FromStringToObjectJSON[*product](`null`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
}

func TestFromObjectToStringJSON(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		got, err := utils.FromObjectToStringJSON(product{ID: 1, Name: "Shoe", Price: 49.99})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `{"id":1,"name":"Shoe","price":49.99}`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("slice", func(t *testing.T) {
		got, err := utils.FromObjectToStringJSON([]string{"a", "b"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != `["a","b"]` {
			t.Errorf("got %s, want [\"a\",\"b\"]", got)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		got, err := utils.FromObjectToStringJSON[*product](nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "null" {
			t.Errorf("got %s, want null", got)
		}
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := utils.FromObjectToStringJSON(make(chan int))
		if err == nil {
			t.Error("expected error for channel, got nil")
		}
	})
}

func TestJSON_RoundTrip(t *testing.T) {
	in := product{ID: 42, Name: "Bag", Price: 120.5, Tags: []string{"leather"}}

	s, err := utils.FromObjectToStringJSON(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := utils.FromStringToObjectJSON[product](s)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch: in %+v, out %+v", in, out)
	}
}

func TestFromStringToObjectYAML_Struct(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    product
		wantErr bool
	}{
		{
			name:  "valid document",
			input: "id: 1\nname: Shoe\nprice: 49.99\ntags:\n  - sale\n  - new\n",
			want:  product{ID: 1, Name: "Shoe", Price: 49.99, Tags: []string{"sale", "new"}},
		},
		{
			name:  "missing fields keep zero values",
			input: "name: Hat\n",
			want:  product{Name: "Hat"},
		},
		{
			name:  "unknown fields are ignored",
			input: "id: 2\nextra: ignored\n",
			want:  product{ID: 2},
		},
		{
			name:  "empty string gives zero value",
			input: "",
			want:  product{},
		},
		{
			name:    "malformed yaml",
			input:   "id: [1, 2\n",
			wantErr: true,
		},
		{
			name:    "wrong field type",
			input:   "id: not-a-number\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.FromStringToObjectYAML[product](tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFromStringToObjectYAML_OtherTypes(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		got, err := utils.FromStringToObjectYAML[map[string]any]("a: 1\nb: x\nc: true\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := map[string]any{"a": 1, "b": "x", "c": true}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("slice", func(t *testing.T) {
		got, err := utils.FromStringToObjectYAML[[]int]("- 1\n- 2\n- 3\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []int{1, 2, 3}) {
			t.Errorf("got %v, want [1 2 3]", got)
		}
	})

	t.Run("json is valid yaml", func(t *testing.T) {
		got, err := utils.FromStringToObjectYAML[product](`{"id": 3, "name": "Cap"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, product{ID: 3, Name: "Cap"}) {
			t.Errorf("got %+v, want {ID:3 Name:Cap}", got)
		}
	})
}

func TestFromObjectToStringYAML(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		got, err := utils.FromObjectToStringYAML(product{ID: 1, Name: "Shoe", Price: 49.99})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "id: 1\nname: Shoe\nprice: 49.99\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("map keys are sorted", func(t *testing.T) {
		got, err := utils.FromObjectToStringYAML(map[string]int{"b": 2, "a": 1, "c": 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "a: 1\nb: 2\nc: 3\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("slice", func(t *testing.T) {
		got, err := utils.FromObjectToStringYAML([]string{"a", "b"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(got) != "- a\n- b" {
			t.Errorf("got %q, want \"- a\\n- b\\n\"", got)
		}
	})

	t.Run("unsupported type returns error instead of panicking", func(t *testing.T) {
		got, err := utils.FromObjectToStringYAML(make(chan int))
		if err == nil {
			t.Fatal("expected error for channel, got nil")
		}
		if !strings.Contains(err.Error(), "recovered from panic") {
			t.Errorf("error = %q, want it to mention the recovered panic", err)
		}
		if got != "" {
			t.Errorf("result = %q, want empty string on panic", got)
		}
	})

	t.Run("marshaler that panics returns error", func(t *testing.T) {
		_, err := utils.FromObjectToStringYAML(panickyYAML{})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("error = %v, want recovered panic containing \"boom\"", err)
		}
	})
}

type panickyYAML struct{}

func (panickyYAML) MarshalYAML() (any, error) { panic("boom") }

type panickyJSON struct{}

func (panickyJSON) MarshalJSON() ([]byte, error) { panic("boom") }

func (*panickyJSON) UnmarshalJSON([]byte) error { panic("boom") }

type panickyYAMLUnmarshal struct{}

func (*panickyYAMLUnmarshal) UnmarshalYAML(func(any) error) error { panic("boom") }

func TestJSON_PanicsBecomeErrors(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		got, err := utils.FromObjectToStringJSON(panickyJSON{})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("error = %v, want recovered panic containing \"boom\"", err)
		}
		if got != "" {
			t.Errorf("result = %q, want empty string on panic", got)
		}
	})

	t.Run("unmarshal", func(t *testing.T) {
		_, err := utils.FromStringToObjectJSON[panickyJSON](`{}`)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("error = %v, want recovered panic containing \"boom\"", err)
		}
	})
}

func TestYAML_UnmarshalPanicBecomesError(t *testing.T) {
	_, err := utils.FromStringToObjectYAML[panickyYAMLUnmarshal]("a: 1\n")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want recovered panic containing \"boom\"", err)
	}
}

func TestYAML_RoundTrip(t *testing.T) {
	in := product{ID: 42, Name: "Bag", Price: 120.5, Tags: []string{"leather"}}

	s, err := utils.FromObjectToStringYAML(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := utils.FromStringToObjectYAML[product](s)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch: in %+v, out %+v", in, out)
	}
}

type parametrisedConfig struct {
	Name     string   `yaml:"name"`
	Port     uint     `yaml:"port"`
	Password string   `yaml:"password"`
	DSN      string   `yaml:"dsn"`
	Note     string   `yaml:"note"`
	Tags     []string `yaml:"tags"`
}

func TestReadParametrisedYAML(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		input   string
		want    parametrisedConfig
		wantErr []string
	}{
		{
			name:  "string and int",
			env:   map[string]string{"T_NAME": "shop", "T_PORT": "3000"},
			input: "name: ${T_NAME}\nport: ${T_PORT}\n",
			want:  parametrisedConfig{Name: "shop", Port: 3000},
		},
		{
			name:  "special characters survive",
			env:   map[string]string{"T_PASS": `a:b #c "d" \e $f ${g}`},
			input: "password: \"${T_PASS}\"\n",
			want:  parametrisedConfig{Password: `a:b #c "d" \e $f ${g}`},
		},
		{
			name:  "several placeholders in one scalar",
			env:   map[string]string{"T_USER": "u", "T_HOST": "h"},
			input: "dsn: ${T_USER}@${T_HOST}:5432\n",
			want:  parametrisedConfig{DSN: "u@h:5432"},
		},
		{
			name:  "sequence items",
			env:   map[string]string{"T_TAG": "new"},
			input: "tags: [sale, \"${T_TAG}\"]\n",
			want:  parametrisedConfig{Tags: []string{"sale", "new"}},
		},
		{
			name:  "empty value is allowed",
			env:   map[string]string{"T_EMPTY": ""},
			input: "name: \"${T_EMPTY}\"\n",
			want:  parametrisedConfig{},
		},
		{
			name:  "bare dollar is left alone",
			input: "note: $HOME and $1\n",
			want:  parametrisedConfig{Note: "$HOME and $1"},
		},
		{
			name:    "missing variables are all listed once",
			input:   "name: ${T_MISSING_A}\npassword: ${T_MISSING_B}\ndsn: ${T_MISSING_A}\n",
			wantErr: []string{"T_MISSING_A, T_MISSING_B"},
		},
		{
			name:  "empty input",
			input: "",
			want:  parametrisedConfig{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := utils.ReadParametrisedYAML[parametrisedConfig](tt.input)
			if len(tt.wantErr) > 0 {
				if !errors.Is(err, utils.ErrEnvVarMissing) {
					t.Fatalf("err = %v, want ErrEnvVarMissing", err)
				}
				for _, part := range tt.wantErr {
					if !strings.Contains(err.Error(), part) {
						t.Fatalf("err = %q, want it to contain %q", err, part)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadParametrisedYAML_MatchesPlainParseWithoutPlaceholders(t *testing.T) {
	input := "id: 7\nname: Hat\nprice: 9.5\ntags: [a, b]\n"
	plain, err := utils.FromStringToObjectYAML[product](input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := utils.ReadParametrisedYAML[product](input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, plain) {
		t.Fatalf("got %+v, want %+v", got, plain)
	}
}

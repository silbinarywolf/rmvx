package rubymarshal

import (
	"encoding/hex"
	"testing"
)

const rubyMarshalHeader = "0408"

func TestString(t *testing.T) {
	var expectedOutput = "EV001"
	// note(jae): 2021-06-13
	// To get Ruby, I load it marshalled files in Sublime Text and
	// copy the hex strings out
	b, err := hex.DecodeString(rubyMarshalHeader + "220A4556303031")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("success cases", func(t *testing.T) {
		// Test using interface
		{
			var v interface{}
			d := NewDecoder(b)
			if err := d.Decode(&v); err != nil {
				t.Fatal(err)
			}
			newV, ok := v.(string)
			if !ok {
				t.Fatalf("expected string not %T", v)
			}
			if newV != expectedOutput {
				t.Fatalf("expected string be \"%s\" but got \"%s\"", expectedOutput, v)
			}
		}

		// Test using string
		{
			var v string
			d := NewDecoder(b)
			if err := d.Decode(&v); err != nil {
				t.Fatal(err)
			}
			if v != expectedOutput {
				t.Fatalf("expected string be \"%s\" but got \"%s\"", expectedOutput, v)
			}
		}
	})
	t.Run("error cases", func(t *testing.T) {
		// Test using int
		{
			var v int
			d := NewDecoder(b)
			err := d.Decode(&v)
			if err == nil {
				t.Fatal("expected an error when passing int to decode string")
			}
			_, ok := err.(*unexpectedType)
			if !ok {
				t.Fatalf("expected \"unexpectedType\" error but got %T", err)
			}
		}
	})
}

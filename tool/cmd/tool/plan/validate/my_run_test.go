package validate

import (
	"bytes"
	"fmt"
	"testing"
)

func TestMyValidate(t *testing.T) {
	var buf bytes.Buffer
	Cmd.SetOut(&buf)
	err := Cmd.RunE(Cmd, []string{"2026-05-07_loop-api-types"})
	fmt.Println("--- OUTPUT START ---")
	fmt.Println(buf.String())
	fmt.Println("--- OUTPUT END ---")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Println("OK")
	}
}

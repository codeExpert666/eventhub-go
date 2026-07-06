package system

import (
	"testing"

	openapigen "eventhub-go/api/openapi/gen"
)

func TestParseEchoCommandKeepsEchoFieldsOriginal(t *testing.T) {
	tag := "  demo  "
	request := openapigen.EchoRequest{
		Message: "  hello  ",
		Tag:     &tag,
	}
	command, appErr := parseEchoCommand(&request)
	if appErr != nil {
		t.Fatalf("parse echo command: %v", appErr)
	}

	if command.Message != "  hello  " {
		t.Fatalf("message = %q, want original message", command.Message)
	}
	if command.Tag == nil || *command.Tag != "  demo  " {
		t.Fatalf("tag = %v, want original tag", command.Tag)
	}
}

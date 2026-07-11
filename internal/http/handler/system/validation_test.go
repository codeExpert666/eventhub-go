package system

import (
	"reflect"
	"strings"
	"testing"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/http/requesterror"
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

func TestParseEchoCommandMapsWhitespaceOnlyMessage(t *testing.T) {
	request := openapigen.EchoRequest{Message: " \t\n"}

	command, appErr := parseEchoCommand(&request)
	if appErr != nil {
		t.Fatalf("parse echo command: %v", appErr)
	}
	if command.Message != request.Message {
		t.Fatalf("message = %q, want original whitespace-only message %q", command.Message, request.Message)
	}
}

func TestParseEchoCommandMapsContractInvalidLengths(t *testing.T) {
	tag := strings.Repeat("t", 33)
	request := openapigen.EchoRequest{
		Message: strings.Repeat("m", 65),
		Tag:     &tag,
	}

	command, appErr := parseEchoCommand(&request)
	if appErr != nil {
		t.Fatalf("parse contract-invalid echo lengths: %v", appErr)
	}
	if command.Message != request.Message || command.Tag == nil || *command.Tag != tag {
		t.Fatalf("echo command = %#v, want overlong fields mapped without validation", command)
	}
}

func TestParseEchoCommandReturnsMalformedBodyForNilRequest(t *testing.T) {
	_, appErr := parseEchoCommand(nil)
	if appErr == nil {
		t.Fatal("parse echo command should reject nil request body")
	}

	want := requesterror.MalformedBody()
	if appErr.Code() != want.Code() || appErr.Message() != want.Message() || !reflect.DeepEqual(appErr.Details(), want.Details()) {
		t.Fatalf("nil request error = %#v, want malformed body error %#v", appErr, want)
	}
}

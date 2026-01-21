package actions

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestIsNotFoundErr(t *testing.T) {
	if !isNotFoundErr(minio.ErrorResponse{Code: "NoSuchKey"}) {
		t.Fatalf("expected NoSuchKey to be not found")
	}
	if isNotFoundErr(errors.New("other")) {
		t.Fatalf("expected other error to not be notfound")
	}
}

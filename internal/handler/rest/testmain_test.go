package rest_test

import (
	"os"
	"testing"

	"github.com/kenyamaneko/overload-party-support/internal/repository/postgres/postgrestest"
)

var sharedPG *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &sharedPG))
}

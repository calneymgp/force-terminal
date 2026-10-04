// SPDX-License-Identifier: Apache-2.0
package wstore

import (
	"context"
	"testing"
)

func TestForceDoesNotOpenLegacyWaveDatabase(t *testing.T) {
	db, err := MakeOldDB(context.Background())
	if db != nil {
		db.Close()
		t.Fatal("Force must not open a Wave database")
	}
	if err == nil {
		t.Fatal("legacy Wave import must be explicitly unavailable")
	}
}

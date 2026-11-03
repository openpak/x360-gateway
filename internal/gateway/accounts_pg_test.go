package gateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/openpak/x360-common/kdc"
	"github.com/openpak/x360-common/pgstate"
)

// The gateway's Postgres store picks up an account the KDC issues after the
// gateway started, and keeps working from its last load if the database goes away.
func TestAccountStoreReloadsFromPostgres(t *testing.T) {
	dsn := os.Getenv("X360_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("X360_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	const schema = "t_gw_accounts"
	t.Cleanup(func() {
		if p, err := pgstate.Open(ctx, dsn, schema); err == nil {
			p.Pool().Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			p.Close()
		}
	})
	kdcSide, err := kdc.OpenPGStore(ctx, dsn, schema, "")
	if err != nil {
		t.Fatal(err)
	}
	defer kdcSide.Close()
	gwSide, err := kdc.OpenPGStore(ctx, dsn, schema, "")
	if err != nil {
		t.Fatal(err)
	}
	defer gwSide.Close()
	g := &Gateway{Accounts: gwSide}
	cert := make([]byte, kdc.CertLen)
	copy(cert[2:7], []byte{4, 4, 4, 4, 4})
	if _, _, err := kdcSide.Provision(cert, time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
	s, err := g.accountStore()
	if err != nil || s.Get(kdc.ConsoleDigits(cert)+"@"+kdc.AccountDomain) == nil {
		t.Fatalf("account issued after start not visible: %v", err)
	}
}

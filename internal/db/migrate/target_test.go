package migrate

import (
 "database/sql"
 "strings"
 "testing"

 _ "modernc.org/sqlite"
)

func TestTargetUpDownAndRepeatedApplication(t *testing.T) {
 db,err:=sql.Open("sqlite",":memory:")
 if err!=nil {t.Fatal(err)}
 defer func() { _ = db.Close() }()
 db.SetMaxOpenConns(1)
 if _,err:=ApplyTarget(db,"0031");err!=nil {t.Fatal(err)}
 if _,err:=ApplyTarget(db,"0032");err!=nil {t.Fatal(err)}
 if _,err:=ApplyTarget(db,"0032");err!=nil {t.Fatal(err)}
 if _,err:=ApplyTarget(db,"0031");err!=nil {t.Fatal(err)}
 var count int
 if err:=db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_mcp_pid'").Scan(&count);err!=nil {t.Fatal(err)}
 if count!=1 {t.Fatal("explicit reversal did not recreate the dropped index")}
 if err:=db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version='0032'").Scan(&count);err!=nil {t.Fatal(err)}
 if count!=0 {t.Fatal("reversed version was not removed from migration history")}
 if _,err:=ApplyTarget(db,"0031");err!=nil {t.Fatal(err)}
 if _,err:=ApplyTarget(db,"0032");err!=nil {t.Fatal(err)}
 if err:=db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_mcp_pid'").Scan(&count);err!=nil {t.Fatal(err)}
 if count!=0 {t.Fatal("reapplying forward migration did not remove the index")}
}

func TestTargetRefusesIrreversibleDowngradeWithoutChanges(t *testing.T) {
 db,err:=sql.Open("sqlite",":memory:")
 if err!=nil {t.Fatal(err)}
 defer func() { _ = db.Close() }()
 db.SetMaxOpenConns(1)
 if _,err:=Apply(db);err!=nil {t.Fatal(err)}
 var before int
 if err:=db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&before);err!=nil {t.Fatal(err)}
 _,err=ApplyTarget(db,"0031")
 if err==nil || !strings.Contains(err.Error(),"not reversible") {t.Fatalf("unsafe downgrade was not refused: %v",err)}
 var after int
 if err:=db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&after);err!=nil {t.Fatal(err)}
 if before!=after {t.Fatalf("downgrade modified migration history: %d -> %d",before,after)}
}

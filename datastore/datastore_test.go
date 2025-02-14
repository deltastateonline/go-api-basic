package datastore

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rs/zerolog"

	"github.com/gilcrest/go-api-basic/domain/errs"
	"github.com/gilcrest/go-api-basic/domain/logger"
)

func TestNewPostgreSQLDSN(t *testing.T) {
	c := qt.New(t)

	got := NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432)

	want := PostgreSQLDSN{
		Host:     "localhost",
		Port:     5432,
		DBName:   "go_api_basic",
		User:     "postgres",
		Password: "",
	}

	c.Assert(got, qt.Equals, want)
}

func TestPostgreSQLDSN_String(t *testing.T) {
	type fields struct {
		Host     string
		Port     int
		DBName   string
		User     string
		Password string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"with password", fields{Host: "localhost", Port: 8080, DBName: "go_api_basic", User: "postgres", Password: "supahsecret"}, "host=localhost port=8080 dbname=go_api_basic user=postgres password=supahsecret sslmode=disable"},
		{"without password", fields{Host: "localhost", Port: 8080, DBName: "go_api_basic", User: "postgres", Password: ""}, "host=localhost port=8080 dbname=go_api_basic user=postgres sslmode=disable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := PostgreSQLDSN{
				Host:     tt.fields.Host,
				Port:     tt.fields.Port,
				DBName:   tt.fields.DBName,
				User:     tt.fields.User,
				Password: tt.fields.Password,
			}
			if got := dsn.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatastore_DB(t *testing.T) {
	c := qt.New(t)

	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	ogdb, cleanup, err := NewPostgreSQLDB(NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432), lgr)
	t.Cleanup(cleanup)
	if err != nil {
		t.Fatal(err)
	}
	ds := Datastore{db: ogdb}
	db := ds.DB()

	c.Assert(db, qt.Equals, ogdb)
}

func TestNewDatastore(t *testing.T) {
	c := qt.New(t)

	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	db, cleanup, err := NewPostgreSQLDB(NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432), lgr)
	t.Cleanup(cleanup)
	if err != nil {
		t.Fatal(err)
	}
	got := NewDatastore(db)

	want := Datastore{db: db}

	c.Assert(got, qt.Equals, want)
}

func TestNewNullInt64(t *testing.T) {
	type args struct {
		i int64
	}
	tests := []struct {
		name string
		args args
		want sql.NullInt64
	}{
		{"has value", args{i: 23}, sql.NullInt64{Int64: 23, Valid: true}},
		{"zero value", args{i: 0}, sql.NullInt64{Int64: 0, Valid: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewNullInt64(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewNullInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatastore_BeginTx(t *testing.T) {

	type fields struct {
		db      *sql.DB
		cleanup func()
	}
	type args struct {
		ctx context.Context
	}

	dsn := NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432)
	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	db, cleanup, dberr := NewPostgreSQLDB(dsn, lgr)
	if dberr != nil {
		t.Errorf("datastore.NewPostgreSQLDB error = %v", dberr)
	}
	ctx := context.Background()
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"typical", fields{db, cleanup}, args{ctx}, false},
		{"closed db", fields{db, cleanup}, args{ctx}, true},
		{"nil db", fields{nil, cleanup}, args{ctx}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &Datastore{
				db: tt.fields.db,
			}
			if tt.wantErr == true {
				tt.fields.cleanup()
			}
			got, err := ds.BeginTx(tt.args.ctx)
			t.Logf("BeginTx error = %v", err)
			if (err != nil) != tt.wantErr {
				t.Errorf("BeginTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ((err != nil) != tt.wantErr) && got == nil {
				t.Errorf("BeginTx() returned nil and should not")
			}
			tt.fields.cleanup()
		})
	}
}

func TestDatastore_RollbackTx(t *testing.T) {
	type fields struct {
		db *sql.DB
	}
	type args struct {
		tx  *sql.Tx
		err error
	}

	dsn := NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432)
	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	db, cleanup, err := NewPostgreSQLDB(dsn, lgr)
	t.Cleanup(cleanup)
	if err != nil {
		t.Errorf("datastore.NewPostgreSQLDB error = %v", err)
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("db.BeginTx error = %v", err)
	}
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("db.BeginTx error = %v", err)
	}
	err = tx2.Commit()
	if err != nil {
		t.Errorf("tx2.Commit() error = %v", err)
	}

	err = errs.E("some error")

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"typical", fields{db}, args{tx, err}},
		{"nil tx", fields{db}, args{nil, err}},
		{"already committed tx", fields{db}, args{tx2, err}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name)
			ds := &Datastore{
				db: tt.fields.db,
			}
			rollbackErr := ds.RollbackTx(tt.args.tx, tt.args.err)
			// RollbackTx only returns an *errs.Error
			e, _ := rollbackErr.(*errs.Error)
			t.Logf("error = %v", e)
			if tt.args.tx == nil && e.Code != "nil_tx" {
				t.Fatalf("ds.RollbackTx() tx was nil, but incorrect error returned = %v", e)
			}
			// I know this is weird, but it's the only way I could think to test this.
			if tt.name == "already committed tx" && e.Code != "rollback_err" {
				t.Fatalf("ds.RollbackTx() tx was already committed, but incorrect error returned = %v", e)
			}
		})
	}
}

func TestDatastore_CommitTx(t *testing.T) {
	type fields struct {
		db *sql.DB
	}
	type args struct {
		tx *sql.Tx
	}
	dsn := NewPostgreSQLDSN("localhost", "go_api_basic", "postgres", "", 5432)
	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	db, cleanup, err := NewPostgreSQLDB(dsn, lgr)
	t.Cleanup(cleanup)
	if err != nil {
		t.Errorf("datastore.NewPostgreSQLDB error = %v", err)
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("db.BeginTx error = %v", err)
	}
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("db.BeginTx error = %v", err)
	}
	err = tx2.Commit()
	if err != nil {
		t.Errorf("tx2.Commit() error = %v", err)
	}
	tx3, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("db.BeginTx error = %v", err)
	}
	err = tx3.Rollback()
	if err != nil {
		t.Errorf("tx2.Commit() error = %v", err)
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"typical", fields{db}, args{tx}, false},
		{"already committed", fields{db}, args{tx2}, true},
		{"already rolled back", fields{db}, args{tx3}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &Datastore{
				db: tt.fields.db,
			}
			if commitErr := ds.CommitTx(tt.args.tx); (commitErr != nil) != tt.wantErr {
				t.Errorf("CommitTx() error = %v, wantErr %v", commitErr, tt.wantErr)
			}
		})
	}
}

func TestNewNullString(t *testing.T) {
	c := qt.New(t)
	type args struct {
		s string
	}

	wantNotNull := sql.NullString{String: "not null", Valid: true}
	wantNull := sql.NullString{String: "", Valid: false}
	tests := []struct {
		name string
		args args
		want sql.NullString
	}{
		{"not null string", args{s: "not null"}, wantNotNull},
		{"null string", args{s: ""}, wantNull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewNullString(tt.args.s)
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

// Probe for research §1.8: modernc.org/sqlite is a pure-Go SQLite, so tacku ships as one static
// binary with no cgo toolchain in the build.
//
// What a run proves: the driver compiles and executes DDL and DML with CGO_ENABLED=0. Note the
// driver name is "sqlite", not the "sqlite3" of the cgo binding.
//
// Run: CGO_ENABLED=0 go run .
package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if _, err := db.Exec(`create table t(id text primary key, title text)`); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`insert into t values ('TSK-1','write the spec')`); err != nil {
		panic(err)
	}
	var title string
	if err := db.QueryRow(`select title from t where id='TSK-1'`).Scan(&title); err != nil {
		panic(err)
	}
	fmt.Println("ok:", title)
}

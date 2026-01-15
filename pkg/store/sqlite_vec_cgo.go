package store

// This file integrates sqlite-vec with mattn/go-sqlite3 by compiling the amalgamation

// #cgo CFLAGS: -DSQLITE_CORE -I${SRCDIR}/../../ -I/home/dsi/go/pkg/mod/github.com/mattn/go-sqlite3@v1.14.33
// #cgo linux LDFLAGS: -lm
// #include "../../sqlite-vec.h"
// #include "../../sqlite-vec.c"
import "C"

// EnableSQLiteVec registers sqlite-vec as an auto-extension for all future database connections
func EnableSQLiteVec() {
	C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_vec_init))
}

// DisableSQLiteVec cancels the sqlite-vec auto-extension
func DisableSQLiteVec() {
	C.sqlite3_cancel_auto_extension((*[0]byte)(C.sqlite3_vec_init))
}

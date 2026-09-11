package types

import (
	"database/sql"
	"fmt"
)

var (
	ParseError = fmt.Errorf("failed to parse the defined env vaariables")
	DataBaseStrEmptyError = fmt.Errorf("invalid parameter, empty dsn value")
)

var DBCONN *sql.DB
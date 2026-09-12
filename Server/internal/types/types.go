package types

import (
	"database/sql"
	"fmt"

	"go.uber.org/zap"
)

var (
	ParseError            = fmt.Errorf("failed to parse the defined env vaariables")
	DataBaseStrEmptyError = fmt.Errorf("invalid parameter, empty dsn value")
)

var DBCONN *sql.DB
var LOG *zap.Logger
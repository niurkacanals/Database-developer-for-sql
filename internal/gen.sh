#!/bin/bash

BASE="mssql mysql oracle postgres sqlite3"

SRC=$(realpath $(cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../)
ALL=$(find $SRC/drivers/ -mindepth 1 -maxdepth 1 -type d|sort)

NL=$'\n'

# generate imports for all drivers
for i in $ALL; do
  MOST=""
  NAME=$(basename $i)
  TAGS="!no_base,!no_$NAME"
  if ! [[ "$BASE" =~ "$NAME" && "$NAME" != "ql" ]]; then
    TAGS="all,!no_$NAME"
    if [[ "$NAME" != "odbc" && "$NAME" != "snowflake" && "$NAME" != "godror" ]]; then
      TAGS="$TAGS most,!no_$NAME"
    fi
    TAGS="$TAGS $NAME,!no_$NAME"
  fi
  DATA=$(cat << ENDSTR
// +build $TAGS

package internal

// Code generated by gen.sh. DO NOT EDIT.

import (
  _ "github.com/xo/usql/drivers/$NAME" // $NAME driver
)
ENDSTR
)
  echo "$DATA" > $SRC/internal/$NAME.go
  gofmt -w -s $SRC/internal/$NAME.go
done

KNOWN=
for i in $ALL; do
  NAME=$(basename $i)
  DRV=$(sed -n '/DRIVER: /p' $i/$NAME.go|sed -e 's/.*DRIVER:\s*//')
  PKG=$(sed -n '/DRIVER: /p' $i/$NAME.go |sed -e 's/^\(\s\|"\|_\)\+//'|sed -e 's/[a-z]\+\s\+"//' |sed -e 's/".*//')
  KNOWN="$KNOWN$NL\"$NAME\": \"$DRV\", // $PKG"
done

DATA=$(cat << ENDSTR
package internal

// Code generated by gen.sh. DO NOT EDIT.

//go:generate ./gen.sh

// KnownBuildTags returns a map of known driver names to its respective build
// tags.
func KnownBuildTags() map[string]string {
  return map[string]string{$KNOWN
  }
}
ENDSTR
)
echo "$DATA" > $SRC/internal/internal.go
gofmt -w -s $SRC/internal/internal.go

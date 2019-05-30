// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"database/sql"
    "fmt"

    _ "github.com/mattn/go-sqlite3"
)

type LoopAttemptOutcome int

const (
	LoopAttemptSuccess		LoopAttemptOutcome = 0
	LoopAttemptNoRoutes		LoopAttemptOutcome = 10
	LoopAttemptFailure		LoopAttemptOutcome = 100
)

type LoopAttempt struct {
	Tstamp int64
	SrcChan uint64
	SrcNode string
	DstChan uint64
	DstNode string
	Amount int64
	FeeLimitRate float64
	Outcome LoopAttemptOutcome
}

func NewLoopAttempt(
	tstamp int64,
	srcChan uint64,
	srcNode string,
	dstChan uint64,
	dstNode string,
	amount int64,
	feeLimitRate float64,
	outcome LoopAttemptOutcome,
) *LoopAttempt {
	return &LoopAttempt{
		Tstamp: tstamp,
		SrcChan: srcChan,
		SrcNode: srcNode,
		DstChan: dstChan,
		DstNode: dstNode,
		Amount: amount,
		FeeLimitRate: feeLimitRate,
		Outcome: outcome,
	}
}

func openDatabase() (*sql.DB) {
    db, err := sql.Open("sqlite3", "./lndtool.db")
	if err != nil {
		panic(fmt.Sprintf("sql.Open failed: %v", err))
	}
	return db
}

func createDatabase(db *sql.DB) {
	cmds := []string{`
        CREATE TABLE IF NOT EXISTS loop_attempt (
	        id INTEGER PRIMARY KEY,
	        tstamp INTEGER,
	        src_chan INTEGER,
	        src_node STRING,
	        dst_chan INTEGER,
	        dst_node STRING,
	        amount INTEGER,
	        fee_limit_rate FLOAT,
	        outcome INTEGER
        )
    `,`
        CREATE INDEX IF NOT EXISTS loop_attempt_tstamp_ndx
            ON loop_attempt(tstamp)
    `,`
        CREATE INDEX IF NOT EXISTS loop_attempt_src_chan_ndx
            ON loop_attempt(src_chan)
    `,`
        CREATE INDEX IF NOT EXISTS loop_attempt_src_node_ndx
            ON loop_attempt(src_node)
    `,`
        CREATE INDEX IF NOT EXISTS loop_attempt_dst_chan_ndx
            ON loop_attempt(dst_chan)
    `,`
        CREATE INDEX IF NOT EXISTS loop_attempt_dst_node_ndx
            ON loop_attempt(dst_node)
    `,}

	for _, cmd := range cmds {
		stmt, err := db.Prepare(cmd)
		if err != nil {
			panic(fmt.Sprintf("db.Prepare \"%s\" failed: %v", cmd, err))
		}
		_, err = stmt.Exec()
		if err != nil {
			panic(fmt.Sprintf("stmt.Exec \"%s\" failed: %v", cmd, err))
		}
	}
}

func insertLoopAttempt(db *sql.DB, attempt *LoopAttempt) {
	cmd := `
        INSERT INTO loop_attempt (
            tstamp,
            src_chan, src_node,
            dst_chan, dst_node,
            amount,
            fee_limit_rate,
            outcome
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `
    stmt, err := db.Prepare(cmd)
	if err != nil {
		panic(fmt.Sprintf("db.Prepare \"%s\" failed: %v", cmd, err))
	}
    _, err = stmt.Exec(
		attempt.Tstamp,
		attempt.SrcChan, attempt.SrcNode,
		attempt.DstChan, attempt.DstNode,
		attempt.Amount,
		attempt.FeeLimitRate,
		attempt.Outcome,
	)
	if err != nil {
		panic(fmt.Sprintf("stmt.Exec \"%s\" failed: %v", cmd, err))
	}
}


// 	
// 	os.Exit(0)
// 	
//     stmt, _ = sql.Prepare(`
//          INSERT INTO people (firstname, lastname) VALUES (?, ?)
//     `)
//     stmt.Exec("Nic", "Raboy")
//     rows, _ := sql.Query("SELECT id, firstname, lastname FROM people")
//     var id int
//     var firstname string
//     var lastname string
//     for rows.Next() {
//         rows.Scan(&id, &firstname, &lastname)
//         fmt.Println(strconv.Itoa(id) + ": " + firstname + " " + lastname)
//     }
// }

func recentlyFailed(db *sql.DB,
	srcChan, dstChan uint64, tstamp int64, amount int64, feeLimitRate float64) bool {
	// Has this loop already failed recently?
	// Don't consider history prior to the horizon.
	// Don't consider higher amounts than this one.
	// Don't consider lessor feeLimitRates than this one.
	query := `
        SELECT COUNT(*) FROM loop_attempt
        WHERE src_chan = ?
          AND dst_chan = ?
          AND tstamp > ?
          AND amount <= ?
          AND fee_limit_rate >= ?
          AND outcome != 0
    `
	row := db.QueryRow(query, srcChan, dstChan, tstamp, amount, feeLimitRate);
	var count int
	switch err := row.Scan(&count); err {
	case sql.ErrNoRows:
		panic("no rows?")
	case nil:
		//
	default:
		panic(err)
	}
	return count > 0
}

type ChannelStats struct {
	RcvCnt int64
	RcvErr int64
	RcvSat int64
	SndCnt int64
	SndErr int64
	SndSat int64
}

func chanStats(db *sql.DB, theChan uint64) *ChannelStats {
	stats := ChannelStats{}
	doChanStats(db, theChan, true, &stats)
	doChanStats(db, theChan, false, &stats)
	return &stats
}

func doChanStats(db *sql.DB, theChan uint64, isRcv bool, retval *ChannelStats) {

	query := `SELECT amount, outcome FROM loop_attempt `
	
	if isRcv {
		query += ` WHERE dst_chan = ?`
	} else {
		query += ` WHERE src_chan = ?`
	}

	rows, err := db.Query(query, theChan)
	if err != nil {
		panic(fmt.Sprintf("db.Query \"%s\" failed: %v", query, err))
	}
	defer rows.Close()
	
	for rows.Next() {
		var amount int
		var outcome int
		err = rows.Scan(&amount, &outcome)
		if err != nil {
			panic(err)
		}
		
		if isRcv {
			retval.RcvCnt += 1
			if outcome != 0 {
				retval.RcvErr += 1
			} else {
				retval.RcvSat += int64(amount)
			}
		} else {
			retval.SndCnt += 1
			if outcome != 0 {
				retval.SndErr += 1
			} else {
				retval.SndSat += int64(amount)
			}
		}
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		panic(err)
	}
}

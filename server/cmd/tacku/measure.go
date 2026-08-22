package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	_ "modernc.org/sqlite"
)

// measure prints the two numbers this product has been waiting on, and nothing else.
//
// Both are recorded already — the surface a status was changed from, and how long a person had been
// away when a visit began — and both were left unanswered for the same reason: nobody has used this
// product, so there is nothing to count. That is a reason to have the question ready, not a reason
// to leave it as a paragraph. A measurement that lives in prose gets re-derived from memory the day
// somebody wants it, and re-derived slightly differently.
//
// It refuses to print a share computed from too little rather than printing one that looks like an
// answer. A percentage over four rows is not a small measurement, it is a wrong one.
func runMeasure(args []string) error {
	return measure(args, os.Stdout)
}

func measure(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("measure", flag.ContinueOnError)
	flags.SetOutput(out)
	path := flags.String("db", "tacku.db", "the database to read")
	least := flags.Int("least", 30, "how many rows a share needs before it is printed at all")
	if err := flags.Parse(args); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", *path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := where(db, out, *least); err != nil {
		return err
	}
	return away(db, out, *least)
}

// where answers B-36: of the status changes a person made, how many came from the board and how
// many from the task screen. Agents are excluded by name rather than by omission — a denominator
// that quietly included them would answer a different question.
func where(db *sql.DB, out io.Writer, least int) error {
	rows, err := db.Query(`select surface, count(*) from changes
		where kind = 'status_moved' and surface in ('board', 'task')
		group by surface`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	counted := map[string]int{}
	total := 0
	for rows.Next() {
		var surface string
		var count int
		if err := rows.Scan(&surface, &count); err != nil {
			return err
		}
		counted[surface] = count
		total += count
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Fprintf(out, "where a person changes a status (B-36)\n")
	fmt.Fprintf(out, "  board %d, task %d, together %d\n", counted["board"], counted["task"], total)
	if total < least {
		fmt.Fprintf(out, "  no share printed: %d moves is not enough to divide, and a percentage over\n"+
			"  that many rows would read like an answer\n", total)
		return nil
	}
	fmt.Fprintf(out, "  from the board: %.0f%%\n", 100*float64(counted["board"])/float64(total))
	return nil
}

// away answers B-38: how long people are actually gone between two visits, which is the number
// domain.DefaultVisitGap was chosen instead of.
func away(db *sql.DB, out io.Writer, least int) error {
	// From `visits`, which keeps a row per arrival, and not from `seen`, which keeps one row per
	// person and overwrites it. The first version of this read `seen` and would have printed a
	// "distribution" made of one latest gap per member — a shape that cannot show the trough the
	// threshold is chosen from, labelled as though it could. The item said so in a line that was
	// read and not acted on.
	rows, err := db.Query(`select away from visits where away > 0 order by away`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer func() { _ = rows.Close() }()

	gaps := []int64{}
	for rows.Next() {
		var seconds int64
		if err := rows.Scan(&seconds); err != nil {
			return err
		}
		gaps = append(gaps, seconds)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Fprintf(out, "how long people are away between visits (B-38)\n")
	fmt.Fprintf(out, "  %d gaps recorded across %d arrivals\n", len(gaps), len(gaps))
	if len(gaps) < least {
		fmt.Fprintf(out, "  no distribution printed: the threshold is chosen from where the gaps fall,\n"+
			"  and %d of them fall nowhere in particular\n", len(gaps))
		return nil
	}
	fmt.Fprintf(out, "  median %s, 90th percentile %s\n",
		hours(gaps[len(gaps)/2]), hours(gaps[(len(gaps)*9)/10]))
	fmt.Fprintf(out, "  the default is 8h; a threshold below the median splits one visit in two\n")
	return nil
}

func hours(seconds int64) string {
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%.1fh", float64(seconds)/3600)
}

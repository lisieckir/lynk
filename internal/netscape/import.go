package netscape

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lisieckir/lynk/internal/bookmark"
)

// Reserved tags used to carry state the Netscape format cannot express.
const (
	TagArchived = "lynk:archived"
	TagUnread   = "lynk:unread"
)

// Import parses r and writes every record for ownerID in one transaction.
// ownerID must be resolved by the caller (CLI: an explicit username or id);
// nothing in the HTML participates in owner selection. No network fetch and no
// metadata enqueue is performed.
func Import(ctx context.Context, db *sql.DB, svc *bookmark.Service, ownerID int64, r io.Reader, opts ImportOptions) (Summary, error) {
	if db == nil || svc == nil {
		return Summary{}, fmt.Errorf("%w: nil database or bookmark service", ErrInvalidOptions)
	}
	if ownerID <= 0 {
		return Summary{}, fmt.Errorf("%w: owner id must be positive", ErrInvalidOptions)
	}
	records, err := Parse(r, opts.Limits)
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Parsed: len(records)}
	inputs := make([]bookmark.CreateInput, len(records))
	for i, rec := range records {
		in, err := createInput(rec)
		if err != nil {
			if !opts.ContinueOnError {
				return Summary{}, fmt.Errorf("record %d: %w", i+1, err)
			}
			sum.Failed++
			continue
		}
		inputs[i] = in
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, in := range inputs {
		if in.URL == "" {
			// A validation-skipped record in ContinueOnError mode.
			continue
		}
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		_, lookupErr := svc.GetByURLInTx(ctx, tx, ownerID, in.URL)
		merge := lookupErr == nil
		switch {
		case merge, errors.Is(lookupErr, bookmark.ErrNotFound):
		default:
			return Summary{}, fmt.Errorf("record %d: lookup URL: %w", i+1, lookupErr)
		}
		if _, err := svc.CreateInTx(ctx, tx, ownerID, in); err != nil {
			return Summary{}, fmt.Errorf("record %d: %w", i+1, err)
		}
		if merge {
			sum.Merged++
		} else {
			sum.Created++
		}
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, fmt.Errorf("commit import transaction: %w", err)
	}
	return sum, nil
}

func createInput(rec Record) (bookmark.CreateInput, error) {
	// Parse already enforces these caps; re-check at the persistence boundary
	// so an oversized value can never reach the database via a future caller.
	if len(rec.Title) > MaxTitleBytes {
		return bookmark.CreateInput{}, fmt.Errorf("%w: title exceeds %d bytes", ErrLimitExceeded, MaxTitleBytes)
	}
	if len(rec.Description) > MaxDescriptionBytes {
		return bookmark.CreateInput{}, fmt.Errorf("%w: description exceeds %d bytes", ErrLimitExceeded, MaxDescriptionBytes)
	}
	raw := strings.TrimSpace(rec.URL)
	if err := bookmark.ValidateURL(raw); err != nil {
		return bookmark.CreateInput{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	tags := make([]string, 0, len(rec.Tags))
	archived, unread := false, false
	for _, tag := range rec.Tags {
		switch tag {
		case TagArchived:
			archived = true
		case TagUnread:
			unread = true
		default:
			tags = append(tags, tag)
		}
	}
	return bookmark.CreateInput{
		URL:          raw,
		Title:        rec.Title,
		Description:  rec.Description,
		Notes:        "",
		Tags:         tags,
		IsArchived:   archived,
		Unread:       unread,
		DateAdded:    rec.DateAdded,
		DateModified: rec.DateModified,
	}, nil
}

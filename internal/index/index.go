// Package index provides BM25 search over corpus paragraphs, backed by an
// SQLite FTS5 table.
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rick-learns/citebound/internal/corpus"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Hit is one search result, best first.
type Hit struct {
	Ref          corpus.Ref
	RFCTitle     string
	SectionTitle string
	Text         string
	Figure       bool
	// Score is the BM25 score; higher is better.
	Score float64
}

// Index searches the paragraphs of the corpus.
type Index struct {
	db *sql.DB
}

const schema = `
CREATE TABLE paragraphs (
	id            INTEGER PRIMARY KEY,
	rfc           INTEGER NOT NULL,
	rfc_title     TEXT    NOT NULL,
	section       TEXT    NOT NULL,
	section_title TEXT    NOT NULL,
	paragraph     INTEGER NOT NULL,
	figure        INTEGER NOT NULL,
	text          TEXT    NOT NULL
);
CREATE VIRTUAL TABLE paragraphs_fts USING fts5(
	text, section_title,
	content = 'paragraphs', content_rowid = 'id',
	tokenize = 'porter unicode61'
);
`

// Column weights for bm25(): a match in the section title counts for more
// than a match in the body, because titles name the topic of every paragraph
// below them.
const (
	weightText         = 1.0
	weightSectionTitle = 3.0
)

// Build creates an in-memory index of every paragraph in rfcs. Paragraphs in
// reference lists are left out: they are bibliography entries, not answers.
func Build(ctx context.Context, rfcs []*corpus.RFC) (*Index, error) {
	// One connection: each connection to ":memory:" is a separate database.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := load(ctx, db, rfcs); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

func load(ctx context.Context, db *sql.DB, rfcs []*corpus.RFC) (err error) {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO paragraphs
		(rfc, rfc_title, section, section_title, paragraph, figure, text)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rfcs {
		for _, s := range r.Sections {
			if isReferenceList(s.Title) {
				continue
			}
			for _, p := range s.Paragraphs {
				if _, err := stmt.ExecContext(ctx, r.Number, r.Title, s.ID, s.Title, p.Ref.Paragraph, p.Figure, p.Text); err != nil {
					return fmt.Errorf("insert %v: %w", p.Ref, err)
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO paragraphs_fts(paragraphs_fts) VALUES ('rebuild')`); err != nil {
		return fmt.Errorf("build full-text index: %w", err)
	}
	return tx.Commit()
}

func isReferenceList(title string) bool {
	return title == "References" || strings.HasSuffix(title, " References")
}

// Close releases the index.
func (ix *Index) Close() error { return ix.db.Close() }

// Len returns the number of indexed paragraphs.
func (ix *Index) Len(ctx context.Context) (int, error) {
	var n int
	err := ix.db.QueryRowContext(ctx, `SELECT count(*) FROM paragraphs`).Scan(&n)
	return n, err
}

// ErrEmptyQuery is returned when a query has no searchable words.
var ErrEmptyQuery = errors.New("query has no searchable words")

// Search returns up to k paragraphs that best match query, best first.
func (ix *Index) Search(ctx context.Context, query string, k int) ([]Hit, error) {
	match := matchExpr(query)
	if match == "" {
		return nil, ErrEmptyQuery
	}
	rows, err := ix.db.QueryContext(ctx, `
		SELECT p.rfc, p.rfc_title, p.section, p.section_title, p.paragraph, p.figure, p.text,
		       bm25(paragraphs_fts, ?, ?) AS rank
		FROM paragraphs_fts
		JOIN paragraphs p ON p.id = paragraphs_fts.rowid
		WHERE paragraphs_fts MATCH ?
		ORDER BY rank, p.id
		LIMIT ?`, weightText, weightSectionTitle, match, k)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		var rank float64
		if err := rows.Scan(&h.Ref.RFC, &h.RFCTitle, &h.Ref.Section, &h.SectionTitle, &h.Ref.Paragraph, &h.Figure, &h.Text, &rank); err != nil {
			return nil, err
		}
		h.Score = -rank // SQLite's bm25() is lower-is-better
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

var wordPattern = regexp.MustCompile(`[\p{L}\p{N}_]+`)

// stopwords are dropped from queries. Without this, "what is the ..." ranks
// paragraphs by how often they say "the". Words that carry meaning in RFCs,
// such as "as" (authorization server) and the RFC 2119 keywords "should" and
// "must", are kept.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "at": true, "be": true,
	"by": true, "can": true, "do": true, "does": true, "for": true, "from": true,
	"how": true, "i": true, "if": true, "in": true, "is": true, "it": true, "of": true,
	"on": true, "or": true, "that": true, "the": true, "this": true,
	"to": true, "what": true, "when": true, "where": true, "which": true, "who": true,
	"why": true, "with": true, "you": true, "my": true, "we": true, "there": true,
}

// matchExpr turns free text into an FTS5 query: every non-stopword term,
// quoted so FTS5 operators in user input are inert, joined with OR.
func matchExpr(query string) string {
	seen := map[string]bool{}
	var terms []string
	for _, w := range wordPattern.FindAllString(strings.ToLower(query), -1) {
		if stopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		terms = append(terms, `"`+w+`"`)
	}
	return strings.Join(terms, " OR ")
}

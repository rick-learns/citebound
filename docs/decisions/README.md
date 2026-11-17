# Decision records

Every decision that is expensive to reverse, or that a new contributor would
otherwise have to rediscover, is written down here. Records are short,
numbered, and never deleted. A decision that changes gets a new record that
supersedes the old one.

| # | Decision | Status |
| --- | --- | --- |
| [0001](0001-record-decisions.md) | Record decisions in `docs/decisions` | Accepted |
| [0002](0002-go-backend-typescript-ui.md) | Go backend, TypeScript UI, one SQLite file | Accepted |

## Writing a record

1. Copy [`template.md`](template.md) to `NNNN-short-title.md` with the next number.
2. Fill in every section. "Alternatives considered" is not optional.
3. Add a row to the table above in the same pull request as the change it
   describes.
4. To change a decision, add a new record, set the old one's status to
   `Superseded by NNNN`, and update the table.

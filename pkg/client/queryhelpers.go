package client

import (
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
)

// Hash validates the provided create table statement and returns its hash.
func (c *Client) Hash(statement string) (string, error) {
	stmt, err := c.parser.ValidateCreateTable(statement, tableland.ChainID(c.chain.ID))
	if err != nil {
		return "", fmt.Errorf("invalid create statement: %s", err)
	}
	return stmt.GetStructureHash(), nil
}

// Validate validates a write query, returning the table id.
func (c *Client) Validate(statement string) (TableID, error) {
	stmts, err := c.parser.ValidateMutatingQuery(statement, tableland.ChainID(c.chain.ID))
	if err != nil {
		return TableID{}, fmt.Errorf("invalid create statement: %s", err)
	}
	return NewTableID(stmts[0].GetTableID().String())
}

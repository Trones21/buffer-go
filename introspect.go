package buffer

import "context"

// introspectionQuery asks GraphQL for the names (and descriptions) of every
// top-level query and mutation. It's the authoritative way to discover what the
// API actually offers without an SDK or reference doc — useful precisely because
// Buffer's schema is what you build against.
const introspectionQuery = `query IntrospectOperations {
  __schema {
    queryType { fields { name description } }
    mutationType { fields { name description } }
  }
}`

// Field is one operation the schema exposes.
type Field struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Schema is the top-level query and mutation operations available to the token.
type Schema struct {
	Queries   []Field
	Mutations []Field
}

// Introspect returns the query and mutation operations the API exposes. Use it
// to discover the real operation names/fields to build against, e.g. from a CLI:
//
//	s, _ := c.Introspect(ctx)
//	for _, f := range s.Mutations { fmt.Println(f.Name) }
func (c *Client) Introspect(ctx context.Context) (*Schema, error) {
	var out struct {
		Schema struct {
			QueryType    struct{ Fields []Field } `json:"queryType"`
			MutationType struct{ Fields []Field } `json:"mutationType"`
		} `json:"__schema"`
	}
	if err := c.Do(ctx, introspectionQuery, nil, &out); err != nil {
		return nil, err
	}
	return &Schema{
		Queries:   out.Schema.QueryType.Fields,
		Mutations: out.Schema.MutationType.Fields,
	}, nil
}

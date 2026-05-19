package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"
)

type FormatFunc func(any) string

type ColumnDef struct {
	Header string
	Key    string // dot-separated path supported via gjson-like lookup; for now plain key
	Format FormatFunc
}

// commandColumns maps "<group>.<verb>" to column definitions for table output.
// Commands without an entry fall back to JSON.
var commandColumns = map[string][]ColumnDef{
	"config.list": {
		{Header: "NAME", Key: "name"},
		{Header: "CLIENT ID", Key: "client_id"},
		{Header: "WRITE", Key: "write"},
		{Header: "BASE URL", Key: "base_url"},
		{Header: "DEFAULT", Key: "default"},
	},
	"auth.scopes.list": {
		{Header: "SCOPE", Key: "scope"},
		{Header: "DESCRIPTION", Key: "description", Format: Truncate80},
	},

	// --- consent ---
	"consent.receipt.list": {
		{Header: "ID", Key: "Id"},
		{Header: "DATA SUBJECT", Key: "DataSubjectIdentifier", Format: Truncate80},
		{Header: "PURPOSE", Key: "PurposeName", Format: Truncate80},
		{Header: "CREATED", Key: "Created"},
	},
	"consent.purpose.list": {
		{Header: "ID", Key: "Id"},
		{Header: "NAME", Key: "Name"},
		{Header: "STATUS", Key: "Status"},
		{Header: "VERSION", Key: "Version"},
	},
	"consent.group.list": {
		{Header: "ID", Key: "Id"},
		{Header: "NAME", Key: "Name"},
		{Header: "STATUS", Key: "Status"},
	},
	"consent.collection-point.list": {
		{Header: "ID", Key: "Id"},
		{Header: "NAME", Key: "Name"},
		{Header: "TYPE", Key: "Type"},
		{Header: "STATUS", Key: "Status"},
	},
	"consent.subject.list": {
		{Header: "ID", Key: "Id"},
		{Header: "IDENTIFIER", Key: "Identifier", Format: Truncate80},
		{Header: "CREATED", Key: "Created"},
	},

	// --- ucpm ---
	"ucpm.subject.list": {
		{Header: "ID", Key: "Id"},
		{Header: "IDENTIFIER", Key: "Identifier", Format: Truncate80},
		{Header: "CREATED", Key: "Created"},
	},

	// --- auth.token ---
	"auth.token.get": {
		{Header: "FIELD", Key: "field"},
		{Header: "VALUE", Key: "value"},
	},

	// --- dsar ---
	"dsar.request.list": {
		{Header: "ID", Key: "id"},
		{Header: "STAGE", Key: "stage"},
		{Header: "TYPE", Key: "requestType"},
		{Header: "SUBJECT", Key: "dataSubjectIdentifier", Format: Truncate80},
		{Header: "CREATED", Key: "createdDate"},
	},
	"dsar.subtask.list": {
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "STATUS", Key: "status"},
	},

	// --- assessment ---
	"assessment.list": {
		{Header: "ID", Key: "assessmentId"},
		{Header: "NAME", Key: "name", Format: Truncate80},
		{Header: "STAGE", Key: "stage"},
		{Header: "TEMPLATE", Key: "templateName", Format: Truncate80},
		{Header: "CREATED", Key: "createdDate"},
	},

	// --- workflow ---
	"workflow.workflow.list": {
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "STATUS", Key: "status"},
	},
	"workflow.task.list": {
		{Header: "ID", Key: "id"},
		{Header: "TITLE", Key: "title", Format: Truncate80},
		{Header: "STATUS", Key: "status"},
		{Header: "ASSIGNEE", Key: "assigneeId"},
	},
	"workflow.approval.list": {
		{Header: "ID", Key: "id"},
		{Header: "SUBJECT", Key: "subject", Format: Truncate80},
		{Header: "STATUS", Key: "status"},
	},

	// --- user / scim ---
	"user.list": {
		{Header: "ID", Key: "id"},
		{Header: "USERNAME", Key: "userName"},
		{Header: "DISPLAY", Key: "displayName"},
		{Header: "ACTIVE", Key: "active"},
	},
	"user.group.list": {
		{Header: "ID", Key: "id"},
		{Header: "DISPLAY", Key: "displayName"},
		{Header: "MEMBERS", Key: "members"},
	},

	// --- role / credential / org ---
	"role.list": {
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "DESCRIPTION", Key: "description", Format: Truncate80},
	},
	"credential.client.list": {
		{Header: "CLIENT ID", Key: "clientId"},
		{Header: "NAME", Key: "name"},
		{Header: "CREATED", Key: "createdDate"},
	},
	"credential.api-key.list": {
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "CREATED", Key: "createdDate"},
	},
	"org.list": {
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "PARENT", Key: "parentId"},
	},

	// --- webhook ---
	"webhook.list": {
		{Header: "ID", Key: "id"},
		{Header: "URL", Key: "url", Format: Truncate80},
		{Header: "EVENTS", Key: "events"},
		{Header: "ACTIVE", Key: "active"},
	},
	"webhook.event.list": {
		{Header: "ID", Key: "id"},
		{Header: "EVENT", Key: "eventType"},
		{Header: "STATUS", Key: "status"},
		{Header: "CREATED", Key: "createdDate"},
	},

	// --- export ---
	"export.bulk.list": {
		{Header: "ID", Key: "id"},
		{Header: "TYPE", Key: "type"},
		{Header: "STATUS", Key: "status"},
		{Header: "CREATED", Key: "createdDate"},
	},

	// --- auditlog ---
	"auditlog.login-history": {
		{Header: "USER", Key: "userId"},
		{Header: "IP", Key: "ipAddress"},
		{Header: "LOCATION", Key: "location", Format: Truncate80},
		{Header: "TIMESTAMP", Key: "timestamp"},
	},
	"auditlog.activity.search": {
		{Header: "USER", Key: "userId"},
		{Header: "ACTION", Key: "action"},
		{Header: "ENTITY", Key: "entityType"},
		{Header: "TIMESTAMP", Key: "timestamp"},
	},
}

// PrintTable renders data using the column definition registered for command.
// Returns an error if no definition exists or the JSON shape is unusable.
func PrintTable(command string, data json.RawMessage) error {
	columns, ok := commandColumns[command]
	if !ok {
		return fmt.Errorf("no table definition for %s", command)
	}

	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		var single map[string]any
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return fmt.Errorf("cannot render %s as table: %w", command, err)
		}
		rows = []map[string]any{single}
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	if term.IsTerminal(int(os.Stdout.Fd())) {
		t.SetStyle(table.StyleLight)
	} else {
		t.SetStyle(table.StyleDefault)
	}

	header := make(table.Row, len(columns))
	for i, col := range columns {
		header[i] = col.Header
	}
	t.AppendHeader(header)

	for _, row := range rows {
		r := make(table.Row, len(columns))
		for i, col := range columns {
			val := row[col.Key]
			if col.Format != nil {
				r[i] = col.Format(val)
			} else {
				r[i] = formatValue(val)
			}
		}
		t.AppendRow(r)
	}

	t.Render()
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(parts, ", ")
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// Truncate80 trims long values to 80 chars + ellipsis for table rendering.
func Truncate80(v any) string {
	s := formatValue(v)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

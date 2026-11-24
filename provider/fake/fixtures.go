package fake

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"time"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

func fixtures() []ticket.Ticket {
	now := time.Now()
	rows := []struct {
		title, status, typ, assignee, priority string
		labels                                 []string
	}{
		{"Login page returns 500 after password reset", "Open", "Bug", "", "High", []string{"auth"}},
		{"Export to CSV drops rows with commas in the name", "Open", "Bug", "", "Medium", []string{"export"}},
		{"Customer asks for invoice copy from last quarter", "Open", "Task", "sam", "Low", nil},
		{"Webhook retries stop after the first failure", "In progress", "Bug", "sam", "High", []string{"integrations", "webhooks"}},
		{"Add a second admin to the account", "In progress", "Task", "alex", "Low", nil},
		{"Mobile app shows stale unread count", "Waiting", "Bug", "alex", "Medium", []string{"mobile"}},
		{"Scheduled report arrives an hour late since the clock change", "Waiting", "Bug", "sam", "Medium", []string{"reports", "timezone"}},
		{"Bulk import rejects files over 10 MB with no message", "Open", "Bug", "", "Medium", []string{"import"}},
		{"Rename a location without losing its history", "Open", "Task", "", "Low", nil},
		{"Two-factor codes are rejected on the second device", "In progress", "Bug", "sam", "High", []string{"auth", "mobile"}},
		{"Search ignores the date filter on the archive tab", "Done", "Bug", "alex", "Low", []string{"search"}},
		{"Set up single sign-on for a new team", "Done", "Task", "sam", "Medium", []string{"auth"}},
	}
	out := make([]ticket.Ticket, 0, len(rows))
	for i, r := range rows {
		n := 101 + i
		key := fmt.Sprintf("DEMO-%d", n)
		t := ticket.Ticket{
			ID:       key,
			Key:      key,
			Title:    r.title,
			Status:   r.status,
			Type:     r.typ,
			Assignee: r.assignee,
			Priority: r.priority,
			URL:      "https://example.com/browse/" + key,
			Created:  now.Add(-time.Duration(len(rows)-i) * 26 * time.Hour),
			Updated:  now.Add(-time.Duration(len(rows)-i) * 3 * time.Hour),
			Body: "## What happened\n\n" + r.title + ".\n\n" +
				"## Steps to reproduce\n\n1. Sign in as a normal user.\n2. Repeat the action from the report.\n3. Compare with the expected result.\n\n" +
				"## Notes\n\nThis ticket is **fixture data** from the `fake` provider.\n\n" +
				"```\nPOST /session 500\nrequest_id=demo-7f3a\n```",
			Fields: map[string]ticket.Value{
				"labels": ticket.List(r.labels...),
			},
			Comments: []ticket.Comment{
				{Author: "reporter", Created: now.Add(-time.Duration(len(rows)-i) * 20 * time.Hour), Body: "Still happening this morning."},
				{Author: "sam", Created: now.Add(-time.Duration(len(rows)-i) * 12 * time.Hour), Body: "I can reproduce it on the **staging** site. Looking into it.\n\n" +
					"```sql\nSELECT id, name, expires_at\nFROM license\n" +
					"WHERE expires_at::date <> (expires_at AT TIME ZONE 'UTC')::date -- rows whose day changes when the stored time is read in another zone, which is every row saved late in the evening\n" +
					"ORDER BY 1 LIMIT 20;\n```"},
				{Author: "alex", Created: now.Add(-time.Duration(len(rows)-i) * 4 * time.Hour), Body: "A fix is in review:\n\n- one change to the handler\n- one new test"},
			},
		}
		// One ticket shows a picture in its body and in a comment.
		if key == PictureTicket {
			t.Attachments = []ticket.Attachment{screenshot}
			t.Body += "\n\nThe page as the reporter saw it:\n\n" + screenshot.Ref()
			t.Comments[0].Body += "\n\n" + screenshot.Ref()
		}
		out = append(out, t)
	}
	return out
}

// fixtureTouches are the changes the first of People made to the fixtures
// before the demo starts.
func fixtureTouches() map[string]touch {
	now := time.Now()
	return map[string]touch{
		"DEMO-104": {now.Add(-2 * time.Hour), provider.ChangeStatus, "Open → In progress"},
		"DEMO-110": {now.Add(-26 * time.Hour), provider.ChangeAssignee, "Assignee – → sam"},
		"DEMO-106": {now.Add(-50 * time.Hour), provider.ChangeTags, "Labels → mobile"},
		"DEMO-112": {now.Add(-4 * 24 * time.Hour), provider.ChangeStatus, "In progress → Done"},
	}
}

// PictureTicket is the fixture ticket that has a picture attached.
const PictureTicket = "DEMO-108"

// screenshot is the attachment of PictureTicket.
var screenshot = ticket.Attachment{ID: "shot-1", Name: "error-page.png", MIME: "image/png"}

// picture draws the file of screenshot: a page with a title bar, a red
// banner and a few lines of grey text, as a PNG.
func picture() []byte {
	const w, h = 480, 270
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(r image.Rectangle, c color.RGBA) {
		draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
	}
	fill(img.Bounds(), color.RGBA{0xf4, 0xf5, 0xf7, 0xff})
	fill(image.Rect(0, 0, w, 36), color.RGBA{0x25, 0x38, 0x58, 0xff})
	fill(image.Rect(40, 70, w-40, 118), color.RGBA{0xde, 0x35, 0x0b, 0xff})
	for i := range 5 {
		y := 146 + i*22
		fill(image.Rect(40, y, w-60-i*55, y+10), color.RGBA{0xb3, 0xba, 0xc5, 0xff})
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

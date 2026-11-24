package ui

import (
	"errors"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/ticket"
)

// maxFormWidth is the width of the form box in a pane with room for it.
const maxFormWidth = 64

// form is the box that asks for typed values: the fields a move requires,
// a field being edited, or a name to search for. It takes the keyboard
// while it is open.
type form struct {
	title string
	f     *huh.Form
	specs []ticket.FieldSpec
	// text and list hold what the form's fields write into, by field name.
	text map[string]*string
	list map[string]*[]string
	done func(map[string]ticket.Value) tea.Cmd
}

// formable reports whether a form can ask for a field of this kind.
func formable(spec ticket.FieldSpec) bool {
	switch spec.Kind {
	case ticket.KindText, ticket.KindRichText, ticket.KindNumber, ticket.KindTags:
		return true
	case ticket.KindSelect, ticket.KindMulti:
		return len(spec.Options) > 0
	}
	return false
}

// openForm shows a form with one field per spec, filled from current. done
// is called with the values when the form is saved.
func (m *Model) openForm(title string, specs []ticket.FieldSpec, current map[string]ticket.Value, done func(map[string]ticket.Value) tea.Cmd) tea.Cmd {
	fm := &form{title: title, specs: specs, text: map[string]*string{}, list: map[string]*[]string{}, done: done}
	var fields []huh.Field
	for _, spec := range specs {
		label := spec.Label
		if label == "" {
			label = m.label(spec.Name)
		}
		names := make([]string, len(spec.Options))
		for i, o := range spec.Options {
			names[i] = o.Name
		}
		cur := current[spec.Name]
		text := cur.Text
		if spec.Kind == ticket.KindTags {
			text = strings.Join(cur.Items, ", ")
		}
		fm.text[spec.Name] = &text

		switch spec.Kind {
		case ticket.KindMulti:
			picked := append([]string{}, cur.Items...)
			fm.list[spec.Name] = &picked
			f := huh.NewMultiSelect[string]().Title(label).Options(huh.NewOptions(names...)...).Value(&picked)
			if spec.Required {
				f.Validate(func(v []string) error {
					if len(v) == 0 {
						return errors.New("pick at least one")
					}
					return nil
				})
			}
			fields = append(fields, f)
		case ticket.KindSelect:
			fields = append(fields, huh.NewSelect[string]().Title(label).Options(huh.NewOptions(names...)...).Value(&text))
		case ticket.KindRichText:
			f := huh.NewText().Title(label).Lines(6).Value(&text)
			if spec.Required {
				f.Validate(huh.ValidateNotEmpty())
			}
			fields = append(fields, f)
		default:
			f := huh.NewInput().Title(label).Value(&text)
			switch spec.Kind {
			case ticket.KindTags:
				f.Description("Separate the values with commas.")
			case ticket.KindNumber:
				f.Validate(func(s string) error {
					if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil && (s != "" || spec.Required) {
						return errors.New("enter a number")
					}
					return nil
				})
			default:
				if spec.Required {
					f.Validate(huh.ValidateNotEmpty())
				}
			}
			fields = append(fields, f)
		}
	}
	fm.f = huh.NewForm(huh.NewGroup(fields...)).WithShowHelp(false)
	m.form = fm
	m.sizeForm()
	return fm.f.Init()
}

// formWidth is the width of the form box in the current pane.
func (m *Model) formWidth() int { return max(min(maxFormWidth, m.w-2), 10) }

// sizeForm tells the open form how much room it has.
func (m *Model) sizeForm() {
	if m.form != nil {
		m.form.f.WithWidth(m.formWidth() - 4)
	}
}

// values reads the form's fields back as ticket values.
func (fm *form) values() map[string]ticket.Value {
	out := map[string]ticket.Value{}
	for _, spec := range fm.specs {
		switch spec.Kind {
		case ticket.KindMulti:
			out[spec.Name] = ticket.Value{Items: append([]string{}, *fm.list[spec.Name]...)}
		case ticket.KindTags:
			items := []string{}
			for _, item := range strings.Split(*fm.text[spec.Name], ",") {
				if item = strings.TrimSpace(item); item != "" {
					items = append(items, item)
				}
			}
			out[spec.Name] = ticket.Value{Items: items}
		default:
			out[spec.Name] = ticket.Text(strings.TrimSpace(*fm.text[spec.Name]))
		}
	}
	return out
}

// formUpdate passes a message to the open form and acts when the form is
// saved or abandoned.
func (m *Model) formUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	fm := m.form
	model, cmd := fm.f.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		fm.f = f
	}
	switch fm.f.State {
	case huh.StateCompleted:
		m.form = nil
		return m, fm.done(fm.values())
	case huh.StateAborted:
		m.form = nil
		return m, nil
	}
	return m, cmd
}

// formLayers draws the form in a box in the middle of the pane, with a
// cancel button on its last line.
func (m *Model) formLayers() []*lipgloss.Layer {
	s := m.styles
	w := m.formWidth()
	inner := w - 4
	lines := []string{s.MenuTitle.Render(fit(m.form.title, inner)), blank(inner, 1)}
	for _, line := range strings.Split(strings.TrimRight(m.form.f.View(), "\n"), "\n") {
		lines = append(lines, pad(ansi.Truncate(line, inner, ""), inner))
	}
	// The box is cut to the pane, keeping the last line for the button.
	if room := m.floor() - 4; len(lines) > room {
		lines = lines[:max(room, 1)]
	}
	lines = append(lines, blank(inner, 1), s.Faint.Render(fit("enter saves, esc cancels", inner)))
	rows := box(lines, w, s.MenuBorder)

	x := max((m.w-w)/2, 0)
	y := max((m.floor()-len(rows))/2, 0)
	button := s.Button.Render("cancel")
	return []*lipgloss.Layer{
		lipgloss.NewLayer(strings.Join(rows, "\n")).ID(idForm).X(x).Y(y).Z(zMenu),
		lipgloss.NewLayer(button).ID(idFormCancel).X(x + w - 2 - lipgloss.Width(button)).Y(y + len(rows) - 2).Z(zMenuItem),
	}
}

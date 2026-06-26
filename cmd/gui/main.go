/*
cmd/gui — Owner: Alexandre.

Fyne-based GUI client for TAP.
Connects directly to the TAP server over TCP (no bridge needed) and
runs two concurrent goroutines:
  - the Fyne event loop on the main goroutine (drives the UI)
  - a network reader goroutine that consumes lines from the server
    and forwards them to the UI via a channel

The UI is composed of panels that the spec requires:
  - room panel (name, description, exits, items, NPCs, players in room)
  - chat panel with three scopes (GLOBAL / ROOM / GROUP)
  - inventory panel
  - action bar (buttons for LOOK, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS)
  - status bar (HP, players in room, players on server)

All commands are formatted via the shared internal/protocol package and
sent over the TCP connection — the GUI holds no game logic, only render
and input wiring.
*/
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"the-answer-protocol/internal/config"
	"the-answer-protocol/internal/protocol"
)

// commandTimeout is how long we wait for a server response before
// assuming the command was lost or the server is out of sync.
const commandTimeout = 5 * time.Second

type pendingCmd struct {
	verb   string
	sentAt time.Time
}

// CommandQueue is a thread-safe FIFO queue to track pending commands.
// This ensures responses are always mapped to the correct command.
type CommandQueue struct {
	mu   sync.Mutex
	cmds []pendingCmd
}

func (q *CommandQueue) Push(verb string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cmds = append(q.cmds, pendingCmd{verb: verb, sentAt: time.Now()})
}

func (q *CommandQueue) Pop() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for len(q.cmds) > 0 {
		item := q.cmds[0]
		q.cmds = q.cmds[1:]

		// If the command is still "fresh", return its verb
		if now.Sub(item.sentAt) < commandTimeout {
			return item.verb
		}
		// Otherwise, log that we pruned it and check the next one
		log.Printf("⚠️ Protocol Sync: Pruned timed-out command '%s'", item.verb)
	}
	return ""
}

func (q *CommandQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.cmds)
}

func (q *CommandQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cmds = nil
}

var (
	// pendingCommands tracks what we sent so we know how to parse what we get back
	pendingCommands = &CommandQueue{}
	// Global connection handle so applyResponse can trigger follow-up commands
	currentGroupID  string // Stores the ID of the group the player is currently in
	myUsername      string
	groupMembers    []string
	globalConn      net.Conn
	activeChatScope = "GLOBAL"
	currentTheme    = themeModern
	currentApp      fyne.App
	// responseHandlers maps a Verb to the logic that handles its OK payload
	responseHandlers map[string]func(payload string)
)

type themeMode string

const (
	themeModern themeMode = "MODERN"
	themeRetro  themeMode = "RETRO"
)

func applyTheme(app fyne.App, mode themeMode) {
	currentTheme = mode
	switch mode {
	case themeRetro:
		app.Settings().SetTheme(&RetroTheme{Theme: theme.DefaultTheme()})
	default:
		currentTheme = themeModern
		app.Settings().SetTheme(&ModernTheme{Theme: theme.DefaultTheme()})
	}
}

func nextThemeLabel() string {
	if currentTheme == themeModern {
		return "RETRO"
	}
	return "MODERN"
}

// main creates the Fyne app, opens the connection to the server, wires
// the read goroutine to the UI, and starts the event loop.
func main() {
	//   1. parse server addr from os.Args (default from config.Port())
	addr := parseServerAddr(os.Args)

	//   2. Create app and Window("TAP")
	a := app.New()
	currentApp = a

	// Apply the default theme, but keep both themes available via toggle.
	applyTheme(a, themeModern)

	w := a.NewWindow("TAP")
	w.SetFixedSize(true)

	// Initialize global widgets used by network handlers early to avoid nil-pointer panics
	// if the server responds before the UI layout is fully rendered.
	statusBusyLabel = canvas.NewText("  ", theme.ForegroundColor())
	statusBusyLabel.TextSize = 24 // Significantly larger than default (usually 14)
	statusHPValue = widget.NewLabel("HP: --")
	statusRoomValue = widget.NewLabel("Room: --")
	statusServerValue = widget.NewLabel("Server: --")
	roomNameValue = widget.NewLabel("--")
	roomDescriptionValue = widget.NewLabel("Waiting for LOOK...")
	roomExitsValue = widget.NewLabel("--")
	roomPlayersValue = widget.NewLabel("--")
	chatGlobalLog = widget.NewLabel("Global chat will appear here...")
	chatGlobalLog.Wrapping = fyne.TextWrapWord
	groupStatusLabel = widget.NewLabel("No Group")
	groupStatusLabel.Wrapping = fyne.TextWrapWord
	chatGlobalScroll = container.NewVScroll(chatGlobalLog)
	chatGlobalScroll.SetMinSize(fyne.NewSize(0, 200))

	chatRoomLog = widget.NewLabel("Room chat will appear here...")
	chatRoomLog.Wrapping = fyne.TextWrapWord
	chatRoomScroll = container.NewVScroll(chatRoomLog)
	chatRoomScroll.SetMinSize(fyne.NewSize(0, 200))

	chatGroupLog = widget.NewLabel("Group chat will appear here...")
	chatGroupLog.Wrapping = fyne.TextWrapWord
	chatGroupScroll = container.NewVScroll(chatGroupLog)
	chatGroupScroll.SetMinSize(fyne.NewSize(0, 200))

	interactionLog = widget.NewLabel("Interaction details will appear here...")
	inventorySelected = widget.NewLabel("Selected item: --")
	moveButtonsContainer = container.NewHBox()
	inventoryData = []protocol.InventoryItem{}

	initResponseHandlers()

	//   3. Channel to communicate between readLoop (Network thread) and the UI
	// using a buffer of up to 100 messages to avoid frezing readLoop
	eventsCh := make(chan string, 100)

	//   4. Dial the server, with clean error handling.
	conn, err := net.Dial("tcp", addr)

	//   5. If connection succeeds, start readLoop.
	if err != nil {
		// log.Fatalf prints the error and calls os.Exit(1) immediately,
		// preventing a nil-pointer crash later down the line.
		log.Fatalf("❌ Critical Error: Failed to connect to server at %s: %v", addr, err)
	} else {
		globalConn = conn

		//   6. goroutine in background to listen to the server thru conn and wire
		// 		eventsCh → UI updates
		go readLoop(conn, eventsCh)

		//   7. Consumer loop: process incoming lines and update UI
		// anonymous function
		go func() {
			// blocking loop, sits waiting for readLoop to drop a new string into eventsCh
			for line := range eventsCh {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Update UI in the main thread context
				fyne.Do(func() {
					if strings.HasPrefix(line, "EVT ") {
						applyEvent(line[4:])
					} else {
						applyResponse(line)
					}
				})
			}
			// () tells Go to immediately invoke (execute)
		}()

		// Safety-net background LOOK. Rooms now also update instantly via the
		// server's EVT ROOM ITEM events, so this slow tick only covers edge cases
		// (and other groups' servers that don't send those events); the long
		// interval keeps the server log clean.
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if globalConn != nil && myUsername != "" {
					logCommand(sendBackgroundCommand(globalConn, "LOOK"))
				}
			}
		}()

		//   8. Login Dialog: Force CONNECT before starting
		// makes the primary game window visible
		w.Show()
		// Instantiates an interactive, editable text input field
		usernameEntry := widget.NewEntry()
		// Sets a faint, gray ghost visual clue text inside the empty text box.
		usernameEntry.SetPlaceHolder("YourName")
		// Popup form: []*widget.FormItem{...}: array containing the rows of the form
		loginForm := dialog.NewForm("Welcome to TAP", "Connect", "Cancel", []*widget.FormItem{
			{Text: "Username", Widget: usernameEntry},
			// The callback function. Code that sits dormant until player clicks one of the two buttons.
		}, func(ok bool) {
			if ok && usernameEntry.Text != "" {
				myUsername = usernameEntry.Text
				logCommand(sendCommand(conn, "CONNECT", usernameEntry.Text))
			} else {
				a.Quit()
			}
			// The parent window (w) this dialog belongs to
		}, w)
		loginForm.Show()
	}

	// Devides the window in 2 columns and creates a vertical window for the right
	// column, that is split in two panels one on top of the other
	mainCenter := container.NewGridWithColumns(2,
		buildRoomPanel(),
		container.NewVBox(
			buildChatPanel(),
			buildInventoryPanel(),
			buildInteractionPanel(),
		),
	)
	//  container.NewBorder(top, bottom, left, right, center)
	w.SetContent(container.NewBorder(buildStatusBar(currentApp), buildActionBar(w, conn), nil, nil, mainCenter))

	// Use Run instead of ShowAndRun since we already called w.Show() to display the dialog
	a.Run()
}

func parseServerAddr(args []string) string {
	if len(args) > 1 && args[1] != "" {
		return args[1]
	}
	return "localhost:" + config.Port()
}

// readLoop reads lines from the server and pushes them into the events
// channel for the UI goroutine to consume. Runs in its own goroutine;
// closes events when the connection ends.
func readLoop(conn net.Conn, events chan<- string) {
	// defered execution LIFO stack
	defer close(events)
	defer conn.Close()

	// reads bytes together until it hits a newline character (\n)
	scanner := bufio.NewScanner(conn)
	// allow reasonably large lines (increase buffer to 256KB)
	// By default, Go's text scanner will crash above 64KB
	const maxTokenSize = 256 * 1024
	// Creates an initial memory allocation slice capable of holding
	//  up to $64KB of text
	buf := make([]byte, 0, 64*1024)
	// Applies this customized memory sizing to the scanner configuration
	// overriding Go's default 64KB limit
	scanner.Buffer(buf, maxTokenSize)

	// Infinite Listening Loop
	for scanner.Scan() {
		line := scanner.Text()
		// send line to events channel; block if UI is slow
		events <- line
	}

	if err := scanner.Err(); err != nil {
		log.Printf("readLoop: scanner error: %v", err)
	}
}

// sendCommand formats a verb + args via internal/protocol and writes it
// to the connection. Called from button click handlers and the chat input.
func sendCommand(conn net.Conn, verb string, args ...string) error {
	if conn == nil {
		return fmt.Errorf("no connection")
	}

	// Initializes a new struct object. Ensure verb is uppercase as per
	//  protocol design
	cmd := protocol.Command{Verb: strings.ToUpper(verb), Args: args}
	if statusBusyLabel != nil {
		statusBusyLabel.Text = "⏳"
		statusBusyLabel.Refresh()
	}
	pendingCommands.Push(cmd.Verb)

	// Calls String method. Takes the verb and the arguments array and stitch
	// them together into a string + "/n"
	line := cmd.String() + "\n"
	// transmits raw data down the network wire to the server.
	// []byte(line): converts (casts) text line into a slice of raw bytes.
	// returns the number of bytes successfully written, and an error object
	_, err := conn.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("sendCommand write: %w", err)
	}
	return nil
}

// sendBackgroundCommand is identical to sendCommand but bypasses setting statusBusyLabel (hourglass icon).
func sendBackgroundCommand(conn net.Conn, verb string, args ...string) error {
	if conn == nil {
		return fmt.Errorf("no connection")
	}

	cmd := protocol.Command{Verb: strings.ToUpper(verb), Args: args}
	pendingCommands.Push(cmd.Verb)

	line := cmd.String() + "\n"
	_, err := conn.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("sendBackgroundCommand write: %w", err)
	}
	return nil
}

var (
	roomNameValue        *widget.Label
	roomDescriptionValue *widget.Label
	roomExitsValue       *widget.Label
	roomItemsList        *widget.List
	roomNPCsList         *widget.List
	roomPlayersValue     *widget.Label
	chatGlobalLog        *widget.Label
	chatRoomLog          *widget.Label
	chatGroupLog         *widget.Label
	chatGlobalScroll     *container.Scroll
	chatRoomScroll       *container.Scroll
	chatGroupScroll      *container.Scroll
	chatTabs             *container.AppTabs
	chatInput            *widget.Entry
	interactionLog       *widget.Label
	interactionScroll    *container.Scroll
	roomItemsData        []protocol.LookItem
	roomNPCsData         []protocol.LookNPC
	roomPlayersData      []string
	inventoryItems       *widget.List
	inventorySelected    *widget.Label
	statusHPValue        *widget.Label
	statusServerValue    *widget.Label
	statusRoomValue      *widget.Label
	groupStatusLabel     *widget.Label
	groupActionContainer *fyne.Container
	statusBusyLabel      *canvas.Text
	moveButtonsContainer *fyne.Container // New: Container for dynamic move buttons
	inventoryData        []protocol.InventoryItem
	selectedRoomItemID   string
	selectedNPCID        string
	selectedItemID       string
)

// buildRoomPanel returns the widget that shows the current room's name,
// description, exits, items, NPCs, and players-in-room. Refreshed on
// every EVT ROOM PRESENCE * and every LOOK response.
func buildRoomPanel() fyne.CanvasObject {
	// Instantiates a label widget displaying generic placeholder text until
	//  the server synchronizes the live data
	// Wraps words into multiple lines when they hit the boundary of the panel container.
	roomNameValue.Wrapping = fyne.TextWrapWord
	roomDescriptionValue.Wrapping = fyne.TextWrapWord
	roomExitsValue.Wrapping = fyne.TextWrapWord

	roomItemsList = widget.NewList(
		func() int { return len(roomItemsData) },
		func() fyne.CanvasObject { return widget.NewLabel("item") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(roomItemsData[i].Name)
		},
	)
	roomItemsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(roomItemsData) {
			selectedRoomItemID = roomItemsData[id].ID
		}
	}
	// We give the list a minimum height so it is visible in the VBox
	roomItemsScroll := container.NewVScroll(roomItemsList)
	roomItemsScroll.SetMinSize(fyne.NewSize(0, 100))

	roomNPCsList = widget.NewList(
		func() int { return len(roomNPCsData) },
		func() fyne.CanvasObject { return widget.NewLabel("npc") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(roomNPCsData[i].Name)
		},
	)
	roomNPCsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(roomNPCsData) {
			selectedNPCID = roomNPCsData[id].ID
		}
	}
	// We give the list a minimum height so it is visible in the VBox
	roomNPCsScroll := container.NewVScroll(roomNPCsList)
	roomNPCsScroll.SetMinSize(fyne.NewSize(0, 100))

	roomPlayersValue.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNameValue,
		widget.NewLabelWithStyle("Description", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomDescriptionValue,
		widget.NewLabelWithStyle("Exits", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomExitsValue,       // This shows the text list of exits
		moveButtonsContainer, // This will hold the buttons for each exit
		widget.NewLabelWithStyle("Items", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomItemsScroll,
		widget.NewLabelWithStyle("NPCs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomNPCsScroll,
		widget.NewLabelWithStyle("Players in room", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		roomPlayersValue,
	)

	// wraps content container inside a widget.NewCard with a border boundary around the
	// panel, global title ("Room") and helper subtitle ("Current room state") at the top.
	return widget.NewCard("Room", "Current room state", content)
}

// buildChatPanel returns the three-tab chat (GLOBAL / ROOM / GROUP)
// plus the input field that builds the right CHAT command.
func buildChatPanel() fyne.CanvasObject {
	chatInput = widget.NewEntry()
	chatInput.SetPlaceHolder("Type a chat message...")

	chatInput.OnSubmitted = func(s string) {
		if s != "" {
			logCommand(sendCommand(globalConn, "CHAT", activeChatScope, s))
			chatInput.SetText("")
		}
	}

	inputRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("SEND", func() {
			if chatInput.Text != "" {
				logCommand(sendCommand(globalConn, "CHAT", activeChatScope, chatInput.Text))
				chatInput.SetText("")
			}
		}),
		chatInput,
	)

	chatTabs = container.NewAppTabs(
		container.NewTabItem("GLOBAL", chatGlobalScroll),
		container.NewTabItem("ROOM", chatRoomScroll),
		container.NewTabItem("GROUP", chatGroupScroll),
	)

	chatTabs.OnSelected = func(t *container.TabItem) {
		activeChatScope = t.Text
		if groupActionContainer != nil {
			if t.Text == "GROUP" {
				groupActionContainer.Show()
			} else {
				groupActionContainer.Hide()
			}
		}
	}

	return widget.NewCard("Chat", "", container.NewVBox(chatTabs, inputRow, groupStatusLabel))
}

// buildInteractionPanel returns a panel for NPC dialogues and combat feedback.
func buildInteractionPanel() fyne.CanvasObject {
	interactionLog.SetText("Interaction details will appear here...")
	interactionLog.Wrapping = fyne.TextWrapWord

	interactionScroll = container.NewVScroll(interactionLog)
	interactionScroll.SetMinSize(fyne.NewSize(0, 150))
	return widget.NewCard("Interaction", "", interactionScroll)
}

// buildInventoryPanel returns the inventory list + a DROP button bound
// to the currently selected item.
func buildInventoryPanel() fyne.CanvasObject {
	inventorySelected.Wrapping = fyne.TextWrapWord

	inventoryItems = widget.NewList(
		func() int { return len(inventoryData) },
		func() fyne.CanvasObject { return widget.NewLabel("item") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(inventoryData[i].Name)
		},
	)

	inventoryItems.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(inventoryData) {
			selectedItemID = inventoryData[id].ID
			inventorySelected.SetText(fmt.Sprintf("Selected item: %s", inventoryData[id].Name))
		}
	}

	inventoryItemsScroll := container.NewVScroll(inventoryItems)
	inventoryItemsScroll.SetMinSize(fyne.NewSize(0, 75)) // Height for ~3 lines

	dropButton := widget.NewButton("DROP", func() {
		if selectedItemID != "" {
			logCommand(sendCommand(globalConn, "DROP", selectedItemID))
		}
	})

	content := container.NewVBox(
		inventoryItemsScroll,
		inventorySelected,
		dropButton,
	)

	return widget.NewCard("Inventory", "", content)
}

func promptAndSend(parent fyne.Window, conn net.Conn, title, placeholder, verb string, prefixArgs ...string) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(placeholder)

	d := dialog.NewForm(title, "Send", "Cancel", []*widget.FormItem{
		{Text: "Input", Widget: entry},
	}, func(ok bool) {
		if !ok {
			return
		}
		value := strings.TrimSpace(entry.Text)
		if value == "" {
			return
		}
		args := append(prefixArgs, value)
		logCommand(sendCommand(conn, verb, args...))
	}, parent)
	d.Show()
}

// buildActionBar returns the row of buttons that map 1:1 to protocol
// verbs (LOOK, WHO, MOVE, TAKE, DROP, TALK, ATTACK, STATUS, QUEST, QUESTS).
// Each button calls sendCommand with the appropriate verb.
func buildActionBar(parent fyne.Window, conn net.Conn) fyne.CanvasObject {
	groupActionContainer = container.NewHBox(
		widget.NewLabel("| Group:"),
		widget.NewButton("CREATE", func() {
			logCommand(sendCommand(globalConn, "GROUP", "CREATE"))
		}),
		widget.NewButton("INVITE", func() {
			promptAndSend(parent, globalConn, "Invite Player", "Player name", "GROUP", "INVITE")
		}),
		widget.NewButton("LEAVE", func() {
			logCommand(sendCommand(globalConn, "GROUP", "LEAVE"))
		}),
	)
	// Start hidden unless the Group tab is somehow already selected
	if activeChatScope != "GROUP" {
		groupActionContainer.Hide()
	}

	return container.NewHBox(
		widget.NewLabelWithStyle("Quick", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButtonWithIcon("LOOK", theme.SearchIcon(), func() { logCommand(sendCommand(conn, "LOOK")) }),
		widget.NewButtonWithIcon("WHO", theme.AccountIcon(), func() { logCommand(sendCommand(conn, "WHO")) }),
		widget.NewButtonWithIcon("STATUS", theme.InfoIcon(), func() { logCommand(sendCommand(conn, "STATUS")) }),
		widget.NewButtonWithIcon("QUIT", theme.WindowCloseIcon(), func() { logCommand(sendCommand(conn, "QUIT")) }),
		layout.NewSpacer(),
		widget.NewLabelWithStyle("Context", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		// MOVE buttons are now dynamically generated in the room panel.
		widget.NewButtonWithIcon("TAKE", theme.ContentAddIcon(), func() {
			if selectedRoomItemID != "" {
				logCommand(sendCommand(conn, "TAKE", selectedRoomItemID))
			}
		}),
		widget.NewButtonWithIcon("TALK", theme.MailComposeIcon(), func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "TALK", selectedNPCID))
			}
		}),
		widget.NewButtonWithIcon("ATTACK", theme.WarningIcon(), func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "ATTACK", selectedNPCID))
			}
		}),
		widget.NewButtonWithIcon("QUEST", theme.DocumentCreateIcon(), func() {
			if selectedNPCID != "" {
				logCommand(sendCommand(conn, "QUEST", selectedNPCID))
			}
		}),
		widget.NewButtonWithIcon("QUESTS", theme.ListIcon(), func() { logCommand(sendCommand(conn, "QUESTS")) }),
		layout.NewSpacer(),
		groupActionContainer,
	)
}

// buildStatusBar returns the top/bottom bar with HP, players in room,
// and players on server. Refreshed on STATUS responses and EVT STATS.
func buildStatusBar(app fyne.App) fyne.CanvasObject {
	var themeToggleButton *widget.Button
	themeToggleButton = widget.NewButton(nextThemeLabel(), func() {
		if currentTheme == themeModern {
			applyTheme(app, themeRetro)
		} else {
			applyTheme(app, themeModern)
		}
		themeToggleButton.SetText(nextThemeLabel())
		themeToggleButton.Refresh()
	})

	return container.NewHBox(
		statusBusyLabel,
		statusHPValue,
		layout.NewSpacer(),
		statusRoomValue,
		layout.NewSpacer(),
		statusServerValue,
		layout.NewSpacer(),
		themeToggleButton,
	)
}

// updateMoveButtons dynamically creates buttons for each available exit.
func updateMoveButtons(exits map[string]string) {
	// Clear existing buttons
	moveButtonsContainer.RemoveAll()
	if len(exits) == 0 {
		moveButtonsContainer.Add(widget.NewLabel("No exits available."))
	} else {
		keys := make([]string, 0, len(exits))
		for k := range exits {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, dir := range keys {
			direction := dir // Capture loop variable for the closure
			btn := widget.NewButton("MOVE "+strings.ToUpper(direction), func() {
				logCommand(sendCommand(globalConn, "MOVE", direction))
			})
			moveButtonsContainer.Add(btn)
		}
	}
	moveButtonsContainer.Refresh() // Important to refresh the container after adding/removing children
	if currentApp != nil && len(currentApp.Driver().AllWindows()) > 0 {
		currentApp.Driver().AllWindows()[0].Content().Refresh()
	}
}

func updateGroupDisplay(groupID string) {
	if groupID == "" || len(groupMembers) == 0 {
		groupStatusLabel.SetText("No Group")
		return
	}
	members := "None"
	if len(groupMembers) > 0 {
		// Sort members for consistent display
		sort.Strings(groupMembers)
		members = strings.Join(groupMembers, ", ")
	}
	groupStatusLabel.SetText(fmt.Sprintf("Group: %s | Members: %s", groupID, members))
}

func logCommand(err error) {
	if err != nil {
		log.Printf("Send command failed: %v", err)
	}
}

func applyResponse(line string) {
	// Every response (OK or ERR) corresponds to the oldest pending command.
	verb := pendingCommands.Pop()

	// If the queue is now empty, hide the activity indicator (with nil check)
	if pendingCommands.Len() == 0 && statusBusyLabel != nil {
		// We use a tiny delay before clearing to ensure the "pulse" is visible
		// to the human eye even on super-fast localhost connections.
		time.AfterFunc(300*time.Millisecond, func() {
			fyne.Do(func() {
				if pendingCommands.Len() == 0 {
					statusBusyLabel.Text = "  "
					statusBusyLabel.Refresh()
				}
			})
		})
	}

	if verb == "" {
		return // No pending command to match this response
	}

	if strings.HasPrefix(line, "ERR ") {
		// Even on error, we popped the verb to stay in sync.
		log.Printf("Server Error for %s: %s", verb, line[4:])
		return
	}

	if !strings.HasPrefix(line, "OK") {
		return
	}

	payload := ""
	if len(line) > 3 {
		payload = line[3:]
	}

	if handler, ok := responseHandlers[verb]; ok {
		handler(payload)
	}
}

// initResponseHandlers populates the map of logic for each protocol verb.
func initResponseHandlers() {
	responseHandlers = map[string]func(string){
		"CONNECT": func(payload string) {
			logCommand(sendBackgroundCommand(globalConn, "LOOK"))
		},
		"QUIT": func(payload string) {
			// Server acknowledged the quit, now we can safely close the app
			fyne.CurrentApp().Quit()
		},
		"MOVE": func(payload string) {
			if strings.HasPrefix(payload, "room=") {
				roomID := strings.TrimPrefix(payload, "room=")
				statusRoomValue.SetText("Room: " + roomID)
				logCommand(sendBackgroundCommand(globalConn, "LOOK"))
			}
		},
		"TAKE": func(payload string) {
			roomItemsList.UnselectAll()
			selectedRoomItemID = ""
			logCommand(sendBackgroundCommand(globalConn, "LOOK"))
		},
		"DROP": func(payload string) {
			inventoryItems.UnselectAll()
			inventorySelected.SetText("Selected item: --")
			selectedItemID = ""
			logCommand(sendBackgroundCommand(globalConn, "LOOK"))
		},
		"LOOK": func(payload string) {
			var data protocol.LookResponse
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				roomNameValue.SetText(data.Room.Name)
				roomDescriptionValue.SetText(data.Room.Description)

				exits := []string{}
				for dir := range data.Room.Exits {
					exits = append(exits, dir)
				}
				roomExitsValue.SetText(strings.Join(exits, ", "))
				statusRoomValue.SetText("Room: " + data.Room.ID)

				sort.Slice(data.Items, func(i, j int) bool { return data.Items[i].ID < data.Items[j].ID })
				roomItemsData = data.Items
				roomItemsList.UnselectAll()
				roomItemsList.Refresh()

				sort.Slice(data.NPCs, func(i, j int) bool { return data.NPCs[i].ID < data.NPCs[j].ID })
				roomNPCsData = data.NPCs
				roomNPCsList.UnselectAll()
				roomNPCsList.Refresh()

				roomPlayersData = data.Players
				roomPlayersValue.SetText(strings.Join(roomPlayersData, ", "))
				updateMoveButtons(data.Room.Exits)

				// Chain refresh: LOOK success triggers INVENTORY sync
				logCommand(sendBackgroundCommand(globalConn, "INVENTORY"))
			}
		},
		"WHO": func(payload string) {
			// Explicitly map to the server's protocol.WhoResponse struct
			var who protocol.WhoResponse
			currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
			formatted := ""

			if err := json.Unmarshal([]byte(payload), &who); err == nil {
				sort.Strings(who.Room)
				playersInRoom := strings.Join(who.Room, "\n - ")
				if len(who.Room) == 0 {
					playersInRoom = "No other players"
				}

				formatted = fmt.Sprintf("\n[WHO] Server Activity:\n - Total connected: %d\n - Active in room (%d):\n - %s\n",
					who.Server, len(who.Room), playersInRoom)
			} else {
				// Fallback to displaying raw payload string if the structure differs
				formatted = fmt.Sprintf("\n[WHO] Online Players:\n%s\n", strings.TrimSpace(payload))
			}

			interactionLog.SetText(currentText + formatted)
			interactionScroll.ScrollToBottom()
		},
		"GROUP": func(payload string) {
			if strings.HasPrefix(payload, "group=") {
				currentGroupID = strings.TrimPrefix(payload, "group=") // Store the actual ID
				groupMembers = []string{myUsername}
				if currentGroupID != myUsername { // Leader is the group ID
					groupMembers = append(groupMembers, currentGroupID)
				}
				updateGroupDisplay(currentGroupID)
				// Immediately send a LOOK command to discover other players in the room
				// who might also be in the group.
				logCommand(sendBackgroundCommand(globalConn, "LOOK"))
			} else if payload == "left" {
				currentGroupID = "" // Clear group ID
				groupMembers = nil
				updateGroupDisplay("")
			} else if payload == "disbanded" { // Server might send this if leader leaves
				currentGroupID = ""
				groupMembers = nil
				updateGroupDisplay("")
			} else if payload == "kicked" { // If player was kicked
				currentGroupID = ""
				groupMembers = nil
				updateGroupDisplay("")
			}
		},
		"STATUS": func(payload string) {
			var data protocol.StatusResponse
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				statusHPValue.SetText(fmt.Sprintf("HP: %d/%d", data.HP, data.MaxHP))
			}
		},
		"INVENTORY": func(payload string) {
			var items []protocol.InventoryItem
			if err := json.Unmarshal([]byte(payload), &items); err == nil {
				sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
				inventoryData = items
				inventoryItems.Refresh()
				// Chain refresh: INVENTORY success triggers STATUS sync
				logCommand(sendBackgroundCommand(globalConn, "STATUS"))
			}
		},
		"ATTACK": func(payload string) {
			var data protocol.AttackResponse
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				formatted := fmt.Sprintf("[Combat] You dealt %d damage. Target HP: %d/%d. Status: %s\n",
					data.Damage, data.TargetHP, data.TargetHP+data.Damage, data.Status)
				currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
				if data.Counter > 0 {
					formatted += fmt.Sprintf("[Combat] NPC countered for %d damage! Your HP: %d\n", data.Counter, data.AttackerHP)
				}
				interactionLog.SetText(currentText + formatted)
				interactionScroll.ScrollToBottom()
				statusHPValue.SetText(fmt.Sprintf("HP: %d/--", data.AttackerHP))

				roomNPCsList.UnselectAll()
				selectedNPCID = ""
			}
		},
		"TALK": func(payload string) {
			var data protocol.TalkResponse
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
				formatted := fmt.Sprintf("[%s]: \"%s\"\n", data.NPC, data.Dialogue)
				interactionLog.SetText(currentText + formatted)
				interactionScroll.ScrollToBottom()

				roomNPCsList.UnselectAll()
				selectedNPCID = ""
			}
		},
		"QUEST": func(payload string) {
			var data protocol.QuestResponse
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
				formatted := fmt.Sprintf("[Quest] %s\nTarget: %s\nReward: %s\nDesc: %s\n",
					data.QuestID, data.Target, data.Reward, data.Description)
				interactionLog.SetText(currentText + formatted)
				interactionScroll.ScrollToBottom()

				roomNPCsList.UnselectAll()
				selectedNPCID = ""
			}
		},
		"QUESTS": func(payload string) {
			var data []protocol.QuestsEntry
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
				formatted := "Your Quests:\n"
				for _, q := range data {
					formatted += fmt.Sprintf("- [%s] %s: %s\n", q.State, q.QuestID, q.Description)
				}
				interactionLog.SetText(currentText + formatted)
				interactionScroll.ScrollToBottom()
			}
		},
		"CHAT": func(payload string) {
			// CHAT commands typically return "OK" with no data.
			// No specific UI update needed here as we wait for EVT CHAT.
		},
	}
}

// applyEvent routes an EVT line to the right panel (chat / room presence
// / group / stats).
func applyEvent(line string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return
	}

	category := parts[0]
	data := parts[1]

	switch category {
	case "STATS":
		statusServerValue.SetText("Server: " + data)
	case "GROUP":
		if strings.HasPrefix(data, "CHAT ") {
			handleChatEvent("GROUP", strings.TrimPrefix(data, "CHAT "))
		} else {
			handleGroupEvent(data)
		}
	case "GLOBAL":
		if strings.HasPrefix(data, "CHAT ") {
			handleChatEvent("GLOBAL", strings.TrimPrefix(data, "CHAT "))
		}
	case "ROOM":
		if strings.HasPrefix(data, "CHAT ") {
			handleChatEvent("ROOM", strings.TrimPrefix(data, "CHAT "))
		} else if strings.HasPrefix(data, "PRESENCE ENTER ") {
			newUser := strings.TrimPrefix(data, "PRESENCE ENTER ")
			exists := false
			for _, u := range roomPlayersData {
				if u == newUser {
					exists = true
					break
				}
			}
			if !exists {
				roomPlayersData = append(roomPlayersData, newUser)
				sort.Strings(roomPlayersData)
				roomPlayersValue.SetText(strings.Join(roomPlayersData, ", "))
			}
			updateChatLog(chatRoomLog, chatRoomScroll, "[System] "+newUser+" entered the room.\n")
		} else if strings.HasPrefix(data, "PRESENCE LEAVE ") {
			oldUser := strings.TrimPrefix(data, "PRESENCE LEAVE ")
			var updated []string
			for _, u := range roomPlayersData {
				if u != oldUser {
					updated = append(updated, u)
				}
			}
			roomPlayersData = updated
			sort.Strings(roomPlayersData)
			roomPlayersValue.SetText(strings.Join(roomPlayersData, ", "))
			updateChatLog(chatRoomLog, chatRoomScroll, "[System] "+oldUser+" left the room.\n")
		} else if strings.HasPrefix(data, "COMBAT ") {
			parts := strings.Split(data, " ")
			if len(parts) >= 5 {
				attacker := parts[1]
				target := parts[2]
				damage := parts[3]
				targetHp := parts[4]

				formatted := fmt.Sprintf("[Combat] %s attacked %s for %s damage! (Target HP: %s)\n", attacker, target, damage, targetHp)
				currentText := strings.TrimPrefix(interactionLog.Text, "Interaction details will appear here...")
				interactionLog.SetText(currentText + formatted)
				interactionScroll.ScrollToBottom()
			}
		} else if strings.HasPrefix(data, "ITEM ") {
			// another player took/dropped something here → refresh the room view
			logCommand(sendBackgroundCommand(globalConn, "LOOK"))
		} else {
			updateChatLog(chatRoomLog, chatRoomScroll, "[System] "+data+"\n")
		}
	}
}

func handleGroupEvent(payload string) {
	parts := strings.SplitN(payload, " ", 2)
	if len(parts) < 2 {
		return
	}

	action := parts[0]
	user := parts[1]

	if action == "INVITE" {
		// Show a dialog to the user
		d := dialog.NewConfirm("Group Invite", fmt.Sprintf("%s invited you to a group. Join?", user), func(ok bool) {
			if ok {
				logCommand(sendCommand(globalConn, "GROUP", "JOIN", user))
			}
		}, fyne.CurrentApp().Driver().AllWindows()[0])
		d.Show()
	} else if action == "JOIN" {
		// Add to internal list if not already there
		exists := false
		for _, m := range groupMembers {
			if m == user {
				exists = true
				break
			}
		}
		if !exists {
			groupMembers = append(groupMembers, user)
		}
		updateChatLog(chatGroupLog, chatGroupScroll, "[System] "+user+" joined the group\n")

		if currentGroupID != "" {
			updateGroupDisplay(currentGroupID)
		}
	} else if action == "LEAVE" {
		updateChatLog(chatGroupLog, chatGroupScroll, "[System] "+user+" left the group\n")

		// Remove from internal list
		for i, m := range groupMembers {
			if m == user {
				groupMembers = append(groupMembers[:i], groupMembers[i+1:]...)
				break
			}
		}

		if currentGroupID != "" && currentGroupID == user { // If the leader left, the group is disbanded
			currentGroupID = "" // Clear group ID
			groupMembers = nil
			updateGroupDisplay("")
		} else if currentGroupID != "" {
			updateGroupDisplay(currentGroupID)
		}
	}
}

func handleChatEvent(scope, payload string) {
	// payload is now: <sender> <message>
	parts := strings.SplitN(payload, " ", 2)
	if len(parts) < 2 {
		return
	}

	sender := parts[0]
	msg := parts[1]

	formatted := fmt.Sprintf("[%s]: %s\n", sender, msg)

	switch scope {
	case "GLOBAL":
		updateChatLog(chatGlobalLog, chatGlobalScroll, formatted)
		if chatTabs != nil {
			chatTabs.SelectIndex(0)
		}
	case "ROOM":
		updateChatLog(chatRoomLog, chatRoomScroll, formatted)
		// Auto-switch to ROOM tab so the player doesn't miss messages
		if chatTabs != nil {
			chatTabs.SelectIndex(1)
		}
	case "GROUP":
		updateChatLog(chatGroupLog, chatGroupScroll, formatted)
		// Discovery heuristic: if someone chats in group, they are a member.
		if currentGroupID != "" {
			found := false
			for _, m := range groupMembers {
				if m == sender {
					found = true
					break
				}
			}
			if !found {
				groupMembers = append(groupMembers, sender)
				updateGroupDisplay(currentGroupID)
			}
		}
		if chatTabs != nil {
			chatTabs.SelectIndex(2)
		}
	}
}

func updateChatLog(label *widget.Label, scroll *container.Scroll, msg string) {
	// If the log still has the initial placeholder, clear it first
	if strings.Contains(label.Text, "will appear here...") {
		label.SetText(msg)
	} else {
		label.SetText(label.Text + msg)
	}
	scroll.ScrollToBottom()
}

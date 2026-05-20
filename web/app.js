/**
 * web/app.js — Owner: Alexandre.
 *
 * Front-end logic for the TAP web GUI.
 * Connects to the cmd/gui WebSocket bridge, sends TAP commands,
 * and renders incoming events and responses in real time.
 *
 * Message flow:
 *   User action → buildCommand() → ws.send() → TAP server
 *   TAP server  → ws.onmessage  → handleMessage() → update DOM
 */

"use strict";

// ── WebSocket connection ──────────────────────────────────────────────────────

/**
 * connect opens a WebSocket to the cmd/gui bridge and wires up all event handlers.
 * Call once on page load.
 * @returns {WebSocket}
 */
function connect() {
  // TODO: implement
  // return new WebSocket("ws://localhost:8080/ws");
}

/**
 * handleMessage parses an incoming server line and routes it to the correct renderer.
 * Lines starting with "EVT" go to handleEvent(); others go to handleResponse().
 * @param {MessageEvent} event
 */
function handleMessage(event) {
  // TODO: implement
}

// ── Sending commands ──────────────────────────────────────────────────────────

/**
 * sendCommand sends a raw TAP command string over the WebSocket.
 * @param {string} cmd  e.g. "MOVE north"
 */
function sendCommand(cmd) {
  // TODO: implement
}

/**
 * buildMoveCommand returns the MOVE command for a given direction button.
 * @param {string} direction  e.g. "north"
 * @returns {string}
 */
function buildMoveCommand(direction) {
  // TODO: implement
  return "";
}

/**
 * buildChatCommand returns the CHAT command for the active tab scope.
 * @param {string} message
 * @returns {string}
 */
function buildChatCommand(message) {
  // TODO: implement
  return "";
}

// ── Handling server responses ─────────────────────────────────────────────────

/**
 * handleResponse handles an "OK …" or "ERR …" response from the server.
 * @param {string} line
 */
function handleResponse(line) {
  // TODO: implement
}

/**
 * handleLookResponse updates the room panel with the JSON payload of a LOOK response.
 * @param {object} data  parsed JSON from "OK {…}"
 */
function handleLookResponse(data) {
  // TODO: implement
}

/**
 * handleInventoryResponse updates the inventory panel with a JSON item array.
 * @param {string[]} items  array of item IDs
 */
function handleInventoryResponse(items) {
  // TODO: implement
}

/**
 * handleStatusResponse updates the HP status bar.
 * @param {object} data  { hp, max_hp, status }
 */
function handleStatusResponse(data) {
  // TODO: implement
}

// ── Handling server events ────────────────────────────────────────────────────

/**
 * handleEvent routes an "EVT …" line to the correct sub-handler.
 * @param {string} line
 */
function handleEvent(line) {
  // TODO: implement — split on space and switch on category + type
}

/**
 * handleRoomPresenceEnter adds a player name to the room players list.
 * @param {string} username
 */
function handleRoomPresenceEnter(username) {
  // TODO: implement
}

/**
 * handleRoomPresenceLeave removes a player name from the room players list.
 * @param {string} username
 */
function handleRoomPresenceLeave(username) {
  // TODO: implement
}

/**
 * appendChatMessage appends a chat line to the correct chat tab (GLOBAL/ROOM/GROUP).
 * @param {string} scope      "GLOBAL" | "ROOM" | "GROUP"
 * @param {string} username
 * @param {string} message
 */
function appendChatMessage(scope, username, message) {
  // TODO: implement
}

/**
 * updatePlayerCount updates the total online player counter in the status bar.
 * @param {number} count
 */
function updatePlayerCount(count) {
  // TODO: implement
}

// ── DOM helpers ───────────────────────────────────────────────────────────────

/**
 * renderRoom updates all room-panel sub-elements from a LOOK JSON payload.
 * @param {object} data  { room, players, items, npcs }
 */
function renderRoom(data) {
  // TODO: implement
}

/**
 * renderExits builds direction buttons from the exits map.
 * @param {object} exits  { north: "room_id", … }
 */
function renderExits(exits) {
  // TODO: implement
}

/**
 * renderInventory rebuilds the inventory list with item names and DROP buttons.
 * @param {string[]} itemIds
 */
function renderInventory(itemIds) {
  // TODO: implement
}

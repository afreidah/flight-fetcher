// -------------------------------------------------------------------------------
// Telegram - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the telegram package.
// -------------------------------------------------------------------------------

// Package telegram delivers notifications to a Telegram chat through the Bot
// API, rendering each notify.Message with HTML markup so the typed fields read
// as a structured block rather than one run of text.
//
// A bot token and a chat ID are both required: the token authenticates the
// sender and the chat ID names the destination, and the bot must already be a
// member of that chat.
//
// Field values are interpolated into the HTML body without escaping. Today
// every value originates from a transponder feed and is constrained to hex
// digits, callsign characters, and formatted coordinates, so none can carry
// markup; a future caller passing free-form text would need escaping added
// first.
//
// telegram implements notify.Notifier and imports notify for the contract. It
// is registered with a notify.Manager by the composition root; nothing else in
// the service references it.
package telegram

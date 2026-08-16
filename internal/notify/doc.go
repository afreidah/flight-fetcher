// -------------------------------------------------------------------------------
// Notify - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the notify package.
// -------------------------------------------------------------------------------

// Package notify defines the notification contract and the fan-out manager that
// delivers a message to every registered backend.
//
// Notifier is the single interface consumers depend on, and Manager satisfies
// it. That composite pattern means the squawk monitor takes one Notifier and
// neither knows nor cares whether zero, one, or several backends are configured
// - adding Telegram alongside Discord is a change to the composition root
// only. Manager collects per-backend failures and reports them together rather
// than stopping at the first, so one broken webhook cannot suppress delivery to
// the others.
//
// Message carries a title, a body, and typed fields rather than pre-rendered
// text, leaving each backend to format for its own medium: Discord renders rich
// embeds, Telegram renders HTML.
//
// notify holds the contract; backend implementations live in its subpackages
// and import it. It imports nothing else from this module.
package notify

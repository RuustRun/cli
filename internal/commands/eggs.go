// This file documents where the 'eggs' command lives. Rather than defining a
// second command with a duplicate name, 'eggs' is registered as an alias of the
// 'ls' command in ls.go (Aliases: []string{"eggs", "list"}), so 'ruust eggs'
// and 'ruust ls' are the same command and print the same Egg table.
//
// It is kept as a thin marker so future maintainers looking for an eggs.go find
// the wiring immediately. There is deliberately no init() and no AddCommand call
// here: adding a separate 'eggs' command would collide with the alias.
package commands

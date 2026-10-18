package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

// setupTerminalModule exposes the terminal surface to Lua:
//
//	local term = require("ttt.terminal")
//	local id, err = term.spawn{ shell = "pi", args = {"--model", "…"}, cwd = ".", label = "pi" }
//	term.send(id, "review this\n")
//	local tail = term.read(id, 4000)   -- raw PTY bytes, ANSI included
//	term.focus(id)
//	for _, t in ipairs(term.list()) do … end
//
// spawn/send/focus require `terminal.control`; read/list require
// `terminal.read`. Missing permissions omit the functions entirely (matching the
// system module), so `if term.spawn then … end` is the capability check.
// Errors follow the system module's convention: (nil, "message").
func setupTerminalModule(L *lua.LState, p *Plugin) {
	loader := func(L *lua.LState) int {
		mod := L.NewTable()
		if p.Granted.TerminalControl {
			L.SetField(mod, "spawn", L.NewFunction(termSpawn(p)))
			L.SetField(mod, "send", L.NewFunction(termSend(p)))
			L.SetField(mod, "focus", L.NewFunction(termFocus(p)))
		}
		if p.Granted.TerminalRead {
			L.SetField(mod, "read", L.NewFunction(termRead(p)))
			L.SetField(mod, "list", L.NewFunction(termList(p)))
		}
		L.Push(mod)
		return 1
	}
	L.PreloadModule("ttt.terminal", loader)
}

func termErr(L *lua.LState, msg string) int {
	L.Push(lua.LNil)
	L.Push(lua.LString(msg))
	return 2
}

// termField reads a string field from a Lua table ("" when absent).
func termField(L *lua.LState, tbl *lua.LTable, key string) string {
	if v, ok := L.GetField(tbl, key).(lua.LString); ok {
		return string(v)
	}
	return ""
}

// termSpawn{ shell = "pi", args = {...}, cwd = "...", label = "pi" } -> id, err
func termSpawn(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		spec := L.OptTable(1, nil)
		if spec == nil {
			return termErr(L, "terminal.spawn expects a table: {shell, args, cwd, label}")
		}
		var args []string
		if t, ok := L.GetField(spec, "args").(*lua.LTable); ok {
			t.ForEach(func(_, v lua.LValue) {
				args = append(args, v.String())
			})
		}
		id, err := p.Terminal.TerminalSpawn(
			termField(L, spec, "shell"),
			args,
			termField(L, spec, "cwd"),
			termField(L, spec, "label"),
		)
		if err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LString(id))
		return 1
	}
}

// termSend(id, text) -> true | nil, err
func termSend(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		text := L.CheckString(2)
		if err := p.Terminal.TerminalSend(id, text); err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LTrue)
		return 1
	}
}

// termRead(id, maxBytes?) -> tail | nil, err
func termRead(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		n := L.OptInt(2, 0)
		tail, err := p.Terminal.TerminalTail(id, n)
		if err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LString(tail))
		return 1
	}
}

// termFocus(id) -> true | nil, err
func termFocus(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		id := L.CheckString(1)
		if err := p.Terminal.TerminalFocus(id); err != nil {
			return termErr(L, err.Error())
		}
		L.Push(lua.LTrue)
		return 1
	}
}

// termList() -> {{id=, label=, cols=, rows=}, …}
func termList(p *Plugin) lua.LGFunction {
	return func(L *lua.LState) int {
		if p.Terminal == nil {
			return termErr(L, "terminal API not available")
		}
		infos := p.Terminal.TerminalList()
		tbl := L.NewTable()
		for _, t := range infos {
			entry := L.NewTable()
			L.SetField(entry, "id", lua.LString(t.ID))
			L.SetField(entry, "label", lua.LString(t.Label))
			L.SetField(entry, "cols", lua.LNumber(t.Cols))
			L.SetField(entry, "rows", lua.LNumber(t.Rows))
			tbl.Append(entry)
		}
		L.Push(tbl)
		return 1
	}
}

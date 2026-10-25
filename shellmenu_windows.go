//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Explorer context menu for files, through the shell's IContextMenu.
// COM objects are apartment-threaded, so the main goroutine is locked to
// its OS thread for the life of the process.

func init() { runtime.LockOSThread() }

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	shParseDisplayName = shell32.NewProc("SHParseDisplayName")
	shBindToParent     = shell32.NewProc("SHBindToParent")
	createPopupMenu    = user32.NewProc("CreatePopupMenu")
	destroyMenu        = user32.NewProc("DestroyMenu")
	getMenuItemCount   = user32.NewProc("GetMenuItemCount")
	getMenuItemInfoW   = user32.NewProc("GetMenuItemInfoW")

	iidIShellFolder  = windows.GUID{Data1: 0x000214E6, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIContextMenu  = windows.GUID{Data1: 0x000214E4, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIContextMenu2 = windows.GUID{Data1: 0x000214F4, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIContextMenu3 = windows.GUID{Data1: 0xBCFCE0A0, Data2: 0xEC17, Data3: 0x11d0, Data4: [8]byte{0x8D, 0x10, 0x00, 0xA0, 0xC9, 0x0F, 0x27, 0x19}}

	comReady bool
)

const (
	coinitApartmentThreaded = 0x2
	miimState               = 0x001
	miimID                  = 0x002
	miimSubmenu             = 0x004
	miimString              = 0x040
	miimFType               = 0x100
	mftSeparator            = 0x800
	mfsDisabled             = 0x003
	wmInitMenuPopup         = 0x0117
	cmicMaskUnicode         = 0x4000
	menuIDFirst             = 1
	menuIDLast              = 0x7FFF
)

// menuItemInfoW mirrors MENUITEMINFOW on 64-bit Windows.
type menuItemInfoW struct {
	cbSize        uint32
	fMask         uint32
	fType         uint32
	fState        uint32
	wID           uint32
	_             uint32
	hSubMenu      uintptr
	hbmpChecked   uintptr
	hbmpUnchecked uintptr
	dwItemData    uintptr
	dwTypeData    *uint16
	cch           uint32
	_             uint32
	hbmpItem      uintptr
}

// cmInvokeCommandInfoEx mirrors CMINVOKECOMMANDINFOEX on 64-bit Windows.
type cmInvokeCommandInfoEx struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       uintptr
	lpParameters uintptr
	lpDirectory  uintptr
	nShow        int32
	dwHotKey     uint32
	hIcon        uintptr
	lpTitle      uintptr
	lpVerbW      uintptr
	lpParamsW    uintptr
	lpDirectoryW *uint16
	lpTitleW     uintptr
	ptInvoke     struct{ x, y int32 }
}

// comCall invokes method idx of a COM object's vtable.
func comCall(obj unsafe.Pointer, idx int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, idx*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

func comRelease(obj unsafe.Pointer) {
	if obj != nil {
		comCall(obj, 2)
	}
}

func hrOK(r uintptr) bool { return r&0x80000000 == 0 }

// shellMenu holds the context menu objects for a selection.
type shellMenu struct {
	dirW  *uint16
	pidls []unsafe.Pointer // absolute PIDLs, freed with CoTaskMemFree
	psf   unsafe.Pointer   // IShellFolder of the parent
	pcm   unsafe.Pointer   // IContextMenu
	pcm2  unsafe.Pointer   // IContextMenu2 (may be nil)
	pcm3  unsafe.Pointer   // IContextMenu3 (may be nil)
	hmenu uintptr
	root  []menuEntry
}

// openShellMenu builds the Explorer context menu for the given files in dir.
func openShellMenu(dir string, names []string) (*shellMenu, error) {
	if !comReady {
		// S_FALSE (already initialised on this thread) is reported as an
		// error by the wrapper; any real problem shows up in the calls below.
		_ = windows.CoInitializeEx(0, coinitApartmentThreaded)
		comReady = true
	}
	m := &shellMenu{}
	var err error
	if m.dirW, err = windows.UTF16PtrFromString(dir); err != nil {
		return nil, err
	}
	var children []unsafe.Pointer
	for _, n := range names {
		p, err := windows.UTF16PtrFromString(dir + `\` + n)
		if err != nil {
			m.Close()
			return nil, err
		}
		var pidl unsafe.Pointer
		if r, _, _ := shParseDisplayName.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0); !hrOK(r) {
			m.Close()
			return nil, fmt.Errorf("SHParseDisplayName(%s): 0x%x", n, r)
		}
		m.pidls = append(m.pidls, pidl)
		var psf, child unsafe.Pointer
		if r, _, _ := shBindToParent.Call(uintptr(pidl), uintptr(unsafe.Pointer(&iidIShellFolder)), uintptr(unsafe.Pointer(&psf)), uintptr(unsafe.Pointer(&child))); !hrOK(r) {
			m.Close()
			return nil, fmt.Errorf("SHBindToParent: 0x%x", r)
		}
		if m.psf == nil {
			m.psf = psf
		} else {
			comRelease(psf)
		}
		children = append(children, child)
	}
	if len(children) == 0 {
		m.Close()
		return nil, errors.New("nothing selected")
	}
	// IShellFolder::GetUIObjectOf is vtable slot 10.
	if r := comCall(m.psf, 10, 0, uintptr(len(children)), uintptr(unsafe.Pointer(&children[0])),
		uintptr(unsafe.Pointer(&iidIContextMenu)), 0, uintptr(unsafe.Pointer(&m.pcm))); !hrOK(r) {
		m.Close()
		return nil, fmt.Errorf("GetUIObjectOf: 0x%x", r)
	}
	// Submenus such as "Send to" are filled on demand through IContextMenu2/3.
	comCall(m.pcm, 0, uintptr(unsafe.Pointer(&iidIContextMenu3)), uintptr(unsafe.Pointer(&m.pcm3)))
	if m.pcm3 == nil {
		comCall(m.pcm, 0, uintptr(unsafe.Pointer(&iidIContextMenu2)), uintptr(unsafe.Pointer(&m.pcm2)))
	}
	m.hmenu, _, _ = createPopupMenu.Call()
	if m.hmenu == 0 {
		m.Close()
		return nil, errors.New("CreatePopupMenu failed")
	}
	// IContextMenu::QueryContextMenu is slot 3.
	if r := comCall(m.pcm, 3, m.hmenu, 0, menuIDFirst, menuIDLast, 0); !hrOK(r) {
		m.Close()
		return nil, fmt.Errorf("QueryContextMenu: 0x%x", r)
	}
	m.root = m.entries(m.hmenu)
	return m, nil
}

func (m *shellMenu) Entries() []menuEntry { return m.root }

// entries reads the items of a menu handle.
func (m *shellMenu) entries(hmenu uintptr) []menuEntry {
	n, _, _ := getMenuItemCount.Call(hmenu)
	count := int(int32(n))
	var out []menuEntry
	for i := 0; i < count; i++ {
		mii := menuItemInfoW{cbSize: uint32(unsafe.Sizeof(menuItemInfoW{})), fMask: miimState | miimID | miimSubmenu | miimString | miimFType}
		if r, _, _ := getMenuItemInfoW.Call(hmenu, uintptr(i), 1, uintptr(unsafe.Pointer(&mii))); r == 0 {
			continue
		}
		e := menuEntry{id: mii.wID, submenu: mii.hSubMenu, hasSub: mii.hSubMenu != 0, index: i, dim: mii.fState&mfsDisabled != 0}
		if mii.fType&mftSeparator != 0 {
			e.sep = true
			out = append(out, e)
			continue
		}
		if mii.cch > 0 {
			buf := make([]uint16, mii.cch+1)
			mii.dwTypeData = &buf[0]
			mii.cch = uint32(len(buf))
			getMenuItemInfoW.Call(hmenu, uintptr(i), 1, uintptr(unsafe.Pointer(&mii)))
			e.text, e.hotkey = menuLabel(windows.UTF16ToString(buf))
		}
		if e.text == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Expand fills a submenu (WM_INITMENUPOPUP through IContextMenu3/2) and
// returns its entries.
func (m *shellMenu) Expand(e menuEntry) []menuEntry {
	if e.submenu == 0 {
		return nil
	}
	lparam := uintptr(uint32(e.index))
	switch {
	case m.pcm3 != nil:
		var lres uintptr
		comCall(m.pcm3, 7, wmInitMenuPopup, e.submenu, lparam, uintptr(unsafe.Pointer(&lres)))
	case m.pcm2 != nil:
		comCall(m.pcm2, 6, wmInitMenuPopup, e.submenu, lparam)
	}
	return m.entries(e.submenu)
}

// Invoke runs a menu command.
func (m *shellMenu) Invoke(e menuEntry) error {
	id := e.id
	info := cmInvokeCommandInfoEx{
		cbSize:       uint32(unsafe.Sizeof(cmInvokeCommandInfoEx{})),
		fMask:        cmicMaskUnicode,
		lpVerb:       uintptr(id - menuIDFirst),
		lpVerbW:      uintptr(id - menuIDFirst),
		lpDirectoryW: m.dirW,
		nShow:        int32(windows.SW_SHOWNORMAL),
	}
	// IContextMenu::InvokeCommand is slot 4.
	if r := comCall(m.pcm, 4, uintptr(unsafe.Pointer(&info))); !hrOK(r) {
		return fmt.Errorf("the command failed (0x%x)", r)
	}
	return nil
}

func (m *shellMenu) Close() {
	if m.hmenu != 0 {
		destroyMenu.Call(m.hmenu)
		m.hmenu = 0
	}
	comRelease(m.pcm3)
	comRelease(m.pcm2)
	comRelease(m.pcm)
	comRelease(m.psf)
	m.pcm3, m.pcm2, m.pcm, m.psf = nil, nil, nil, nil
	for _, p := range m.pidls {
		windows.CoTaskMemFree(p)
	}
	m.pidls = nil
}

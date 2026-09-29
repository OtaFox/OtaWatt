//go:build windows

package main

import (
	"context"
	"embed"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	appName    = "OtaWatt"
	appCredit  = "(créé par OtaFox)"
	appVersion = "0.5.0"

	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_COMMAND        = 0x0111
	WM_DRAWITEM       = 0x002B
	WM_CTLCOLORBTN    = 0x0135
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORSTATIC = 0x0138
	WM_SETFONT        = 0x0030
	WM_SETICON        = 0x0080
	WM_APP            = 0x8000
	WM_APP_DATA       = WM_APP + 1
	WM_APP_TRAY       = WM_APP + 2

	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	SW_HIDE       = 0
	SW_SHOW       = 5
	SW_SHOWNORMAL = 1

	AW_HIDE     = 0x00010000
	AW_ACTIVATE = 0x00020000
	AW_BLEND    = 0x00080000
	FADE_MS     = 300

	WS_OVERLAPPED  = 0x00000000
	WS_CAPTION     = 0x00C00000
	WS_SYSMENU     = 0x00080000
	WS_MINIMIZEBOX = 0x00020000
	WS_VISIBLE     = 0x10000000
	WS_CHILD       = 0x40000000
	WS_TABSTOP     = 0x00010000
	WS_BORDER      = 0x00800000

	WS_EX_APPWINDOW  = 0x00040000
	WS_EX_CLIENTEDGE = 0x00000200
	WS_EX_LAYERED    = 0x00080000

	SS_CENTER      = 0x00000001
	SS_ICON        = 0x00000003
	SS_CENTERIMAGE = 0x00000200

	ES_CENTER      = 0x00000001
	ES_AUTOHSCROLL = 0x00000080

	BS_OWNERDRAW    = 0x0000000B
	BS_AUTOCHECKBOX = 0x00000003
	BS_MULTILINE    = 0x00002000

	BM_GETCHECK   = 0x00F0
	BM_SETCHECK   = 0x00F1
	BST_UNCHECKED = 0
	BST_CHECKED   = 1

	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	HWND_TOPMOST   = ^uintptr(0) // -1
	HWND_NOTOPMOST = ^uintptr(1) // -2

	NIM_ADD     = 0x00000000
	NIM_MODIFY  = 0x00000001
	NIM_DELETE  = 0x00000002
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800
	MF_CHECKED   = 0x00000008
	MF_GRAYED    = 0x00000001

	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	COLOR_WINDOW = 5
	IDC_ARROW    = 32512

	TRANSPARENT   = 1
	DT_CENTER     = 0x00000001
	DT_VCENTER    = 0x00000004
	DT_SINGLELINE = 0x00000020

	ODS_SELECTED = 0x0001
	ODS_DISABLED = 0x0004

	LWA_ALPHA = 0x00000002

	FULL_WIDTH     = 500
	FULL_HEIGHT    = 700
	COMPACT_WIDTH  = 320
	COMPACT_HEIGHT = 300
	COMPACT_ALPHA  = 179 // environ 70 % de 255

	ID_ICON     = 100
	ID_TITLE    = 101
	ID_CREDIT   = 102
	ID_POWER    = 103
	ID_VOLTAGE  = 104
	ID_CURRENT  = 105
	ID_FREQ     = 106
	ID_TEMP     = 107
	ID_HEALTH   = 108
	ID_STATUS   = 109
	ID_START    = 110
	ID_STOP     = 111
	ID_TOPMOST  = 112
	ID_SETTINGS = 113
	ID_OPTIONS  = 114
	ID_COMPACT  = 115

	ID_C_POWER   = 116
	ID_C_VOLTAGE = 117
	ID_C_CURRENT = 118
	ID_C_TEMP    = 119
	ID_C_HEALTH  = 120
	ID_C_BACK    = 121

	ID_CFG_INFO     = 200
	ID_CFG_LABEL    = 201
	ID_CFG_EDIT     = 202
	ID_CFG_TEST     = 203
	ID_CFG_STATUS   = 204
	ID_CFG_CONTINUE = 205
	ID_CFG_BACK     = 206

	ID_SET_INFO         = 220
	ID_SET_TOPMOST      = 221
	ID_SET_STARTUP      = 222
	ID_SET_STARTUP_NOTE = 223
	ID_SET_SHELLY       = 224
	ID_SET_LOGS         = 225
	ID_SET_BACK         = 226
	ID_SET_STATUS       = 227
	ID_SET_SEPARATOR    = 228
	ID_SET_STARTMIN     = 229

	MENU_OPEN     = 3001
	MENU_RECORD   = 3002
	MENU_STOP     = 3003
	MENU_TOPMOST  = 3004
	MENU_SETTINGS = 3005
	MENU_LOGS     = 3006
	MENU_EXIT     = 3007

	STM_SETICON = 0x0170

	LR_DEFAULTCOLOR = 0x0000

	ERROR_ALREADY_EXISTS      = 183
	ERROR_INSUFFICIENT_BUFFER = 122
	APPMODEL_ERROR_NO_PACKAGE = 15700
)

//go:embed OtaWatt.ico Audiowide-Regular.ttf
var embeddedFS embed.FS

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     RECT
	ItemData   uintptr
}
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         GUID
	HBalloonIcon     uintptr
}

type Config struct {
	ShellyAddress  string `json:"shelly_address"`
	TopMost        bool   `json:"top_most"`
	StartMinimized bool   `json:"start_minimized"`
}

type SwitchStatus struct {
	ID      int      `json:"id"`
	Output  bool     `json:"output"`
	APower  float64  `json:"apower"`
	Voltage float64  `json:"voltage"`
	Freq    float64  `json:"freq"`
	Current float64  `json:"current"`
	Errors  []string `json:"errors"`
	AEnergy struct {
		Total float64 `json:"total"`
	} `json:"aenergy"`
	Temperature struct {
		TC float64 `json:"tC"`
		TF float64 `json:"tF"`
	} `json:"temperature"`
}

type appState struct {
	hwnd         uintptr
	controls     map[int]uintptr
	buttonAccent map[uintptr]uint32
	staticColor  map[uintptr]uint32

	hIcon          uintptr
	bgBrush        uintptr
	panelBrush     uintptr
	pressedBrush   uintptr
	separatorBrush uintptr

	fontTitle       uintptr
	fontCredit      uintptr
	fontPower       uintptr
	fontMeasure     uintptr
	fontHealth      uintptr
	fontNormal      uintptr
	fontSmall       uintptr
	fontButton      uintptr
	fontButtonSmall uintptr
	fontEdit        uintptr
	fontMemHandle   uintptr
	fontMemData     []byte

	cfg                  Config
	configPath           string
	logsDir              string
	configured           bool
	showingConfig        bool
	showingOptions       bool // écran Paramètres
	configReturnSettings bool
	compact              bool
	testedAddress        string

	mu           sync.Mutex
	latest       *SwitchStatus
	lastErr      error
	recording    bool
	recFile      *os.File
	recCSV       *csv.Writer
	recStart     time.Time
	recPending   int
	recLastFlush time.Time

	httpClient *http.Client
	pollCancel context.CancelFunc
	quitting   bool
}

var app appState
var startupLaunch bool

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")

	pRegisterClassExW           = user32.NewProc("RegisterClassExW")
	pCreateWindowExW            = user32.NewProc("CreateWindowExW")
	pDefWindowProcW             = user32.NewProc("DefWindowProcW")
	pShowWindow                 = user32.NewProc("ShowWindow")
	pIsWindowVisible            = user32.NewProc("IsWindowVisible")
	pUpdateWindow               = user32.NewProc("UpdateWindow")
	pGetMessageW                = user32.NewProc("GetMessageW")
	pTranslateMessage           = user32.NewProc("TranslateMessage")
	pDispatchMessageW           = user32.NewProc("DispatchMessageW")
	pPostQuitMessage            = user32.NewProc("PostQuitMessage")
	pPostMessageW               = user32.NewProc("PostMessageW")
	pSendMessageW               = user32.NewProc("SendMessageW")
	pSetWindowTextW             = user32.NewProc("SetWindowTextW")
	pGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	pGetWindowTextW             = user32.NewProc("GetWindowTextW")
	pEnableWindow               = user32.NewProc("EnableWindow")
	pSetWindowPos               = user32.NewProc("SetWindowPos")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	pLoadCursorW                = user32.NewProc("LoadCursorW")
	pGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	pSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	pDestroyWindow              = user32.NewProc("DestroyWindow")
	pCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	pAppendMenuW                = user32.NewProc("AppendMenuW")
	pTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                = user32.NewProc("DestroyMenu")
	pGetCursorPos               = user32.NewProc("GetCursorPos")
	pDrawTextW                  = user32.NewProc("DrawTextW")
	pFillRect                   = user32.NewProc("FillRect")
	pFrameRect                  = user32.NewProc("FrameRect")
	pCreateIconFromResourceEx   = user32.NewProc("CreateIconFromResourceEx")
	pDestroyIcon                = user32.NewProc("DestroyIcon")
	pAnimateWindow              = user32.NewProc("AnimateWindow")

	pCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject            = gdi32.NewProc("DeleteObject")
	pSetTextColor            = gdi32.NewProc("SetTextColor")
	pSetBkColor              = gdi32.NewProc("SetBkColor")
	pSetBkMode               = gdi32.NewProc("SetBkMode")
	pCreateFontW             = gdi32.NewProc("CreateFontW")
	pAddFontMemResourceEx    = gdi32.NewProc("AddFontMemResourceEx")
	pRemoveFontMemResourceEx = gdi32.NewProc("RemoveFontMemResourceEx")

	pGetModuleHandleW          = kernel32.NewProc("GetModuleHandleW")
	pGetCurrentPackageFullName = kernel32.NewProc("GetCurrentPackageFullName")
	pCreateMutexW              = kernel32.NewProc("CreateMutexW")
	pCloseHandle               = kernel32.NewProc("CloseHandle")

	pShellNotifyIconW     = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW        = shell32.NewProc("ShellExecuteW")
	pSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")

	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	pCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	pCoUninitialize   = ole32.NewProc("CoUninitialize")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
)

func rgb(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

var (
	colBG      = rgb(10, 6, 18)
	colPanel   = rgb(24, 12, 38)
	colPressed = rgb(40, 18, 58)
	colViolet  = rgb(176, 38, 255)
	colGreen   = rgb(57, 255, 20)
	colMagenta = rgb(255, 43, 214)
	colWhite   = rgb(244, 242, 250)
	colDim     = rgb(205, 195, 220)
	colOrange  = rgb(255, 176, 0)
	colRed     = rgb(255, 59, 107)
)

func u16(s string) *uint16    { p, _ := syscall.UTF16PtrFromString(s); return p }
func loword(v uintptr) uint16 { return uint16(v & 0xffff) }

func setText(hwnd uintptr, text string) {
	pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(u16(text))))
}
func getText(hwnd uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(hwnd)
	buf := make([]uint16, int(n)+1)
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func show(hwnd uintptr, yes bool) {
	cmd := uintptr(SW_HIDE)
	if yes {
		cmd = SW_SHOW
	}
	pShowWindow.Call(hwnd, cmd)
}
func enable(hwnd uintptr, yes bool) {
	v := uintptr(0)
	if yes {
		v = 1
	}
	pEnableWindow.Call(hwnd, v)
}

func loadEmbeddedAudiowide() bool {
	b, err := embeddedFS.ReadFile("Audiowide-Regular.ttf")
	if err != nil || len(b) == 0 {
		return false
	}
	var count uint32
	h, _, _ := pAddFontMemResourceEx.Call(
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, uintptr(unsafe.Pointer(&count)),
	)
	if h == 0 || count == 0 {
		return false
	}
	app.fontMemHandle = h
	app.fontMemData = b // garde les octets vivants pendant toute la durée de l'application
	return true
}

func createFontFace(height int32, weight int32, face string) uintptr {
	name := u16(face)
	h, _, _ := pCreateFontW.Call(
		uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1, 0, 0, 5, 0, uintptr(unsafe.Pointer(name)),
	)
	return h
}

func createFont(height int32, weight int32) uintptr {
	return createFontFace(height, weight, "Segoe UI")
}

func setWindowAlpha(alpha byte) {
	pSetLayeredWindowAttributes.Call(app.hwnd, 0, uintptr(alpha), LWA_ALPHA)
}

func setWindowSize(w, h int32) {
	pSetWindowPos.Call(app.hwnd, 0, 0, 0, uintptr(w), uintptr(h), SWP_NOMOVE|SWP_NOZORDER)
}

func fullOpacity()    { setWindowAlpha(255) }
func compactOpacity() { setWindowAlpha(COMPACT_ALPHA) }

func createControl(class, text string, style, exStyle uint32, x, y, w, h int32, id int) uintptr {
	hwnd, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(u16(class))),
		uintptr(unsafe.Pointer(u16(text))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		app.hwnd, uintptr(id), 0, 0,
	)
	app.controls[id] = hwnd
	return hwnd
}

func setFont(hwnd, font uintptr) { pSendMessageW.Call(hwnd, WM_SETFONT, font, 1) }

func addStatic(id int, text string, x, y, w, h int32, font uintptr, color uint32) uintptr {
	hwnd := createControl("STATIC", text, WS_CHILD|WS_VISIBLE|SS_CENTER, 0, x, y, w, h, id)
	setFont(hwnd, font)
	app.staticColor[hwnd] = color
	return hwnd
}

func addButton(id int, text string, x, y, w, h int32, accent uint32) uintptr {
	hwnd := createControl("BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, x, y, w, h, id)
	setFont(hwnd, app.fontButton)
	app.buttonAccent[hwnd] = accent
	return hwnd
}

func addCheckbox(id int, text string, x, y, w, h int32) uintptr {
	hwnd := createControl("BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 0, x, y, w, h, id)
	setFont(hwnd, app.fontNormal)
	return hwnd
}

func addCheckboxMultiline(id int, text string, x, y, w, h int32) uintptr {
	hwnd := createControl("BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX|BS_MULTILINE, 0, x, y, w, h, id)
	setFont(hwnd, app.fontNormal)
	return hwnd
}

func setChecked(hwnd uintptr, checked bool) {
	state := uintptr(BST_UNCHECKED)
	if checked {
		state = BST_CHECKED
	}
	pSendMessageW.Call(hwnd, BM_SETCHECK, state, 0)
}

func isChecked(hwnd uintptr) bool {
	r, _, _ := pSendMessageW.Call(hwnd, BM_GETCHECK, 0, 0)
	return r == BST_CHECKED
}

func addEdit(id int, text string, x, y, w, h int32) uintptr {
	hwnd := createControl("EDIT", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_CENTER|ES_AUTOHSCROLL, WS_EX_CLIENTEDGE, x, y, w, h, id)
	setFont(hwnd, app.fontEdit)
	return hwnd
}

func drawOwnerButton(dis *DRAWITEMSTRUCT) {
	brush := app.panelBrush
	if dis.ItemState&ODS_SELECTED != 0 {
		brush = app.pressedBrush
	}
	pFillRect.Call(dis.HDC, uintptr(unsafe.Pointer(&dis.RcItem)), brush)

	accent := app.buttonAccent[dis.HwndItem]
	border, _, _ := pCreateSolidBrush.Call(uintptr(accent))
	pFrameRect.Call(dis.HDC, uintptr(unsafe.Pointer(&dis.RcItem)), border)
	pDeleteObject.Call(border)

	color := colWhite
	if dis.ItemState&ODS_DISABLED != 0 {
		color = rgb(110, 105, 120)
	}
	pSetTextColor.Call(dis.HDC, uintptr(color))
	pSetBkMode.Call(dis.HDC, TRANSPARENT)

	txt := getText(dis.HwndItem)
	pDrawTextW.Call(dis.HDC, uintptr(unsafe.Pointer(u16(txt))), ^uintptr(0), uintptr(unsafe.Pointer(&dis.RcItem)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func iconFromICO(data []byte, cx, cy int32) (uintptr, error) {
	if len(data) < 6 {
		return 0, errors.New("icône invalide")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count < 1 || len(data) < 6+16*count {
		return 0, errors.New("icône invalide")
	}
	type entry struct {
		w, h      int
		size, off uint32
	}
	entries := make([]entry, 0, count)
	for i := 0; i < count; i++ {
		p := 6 + i*16
		w := int(data[p])
		if w == 0 {
			w = 256
		}
		h := int(data[p+1])
		if h == 0 {
			h = 256
		}
		size := binary.LittleEndian.Uint32(data[p+8 : p+12])
		off := binary.LittleEndian.Uint32(data[p+12 : p+16])
		if int(off+size) <= len(data) {
			entries = append(entries, entry{w, h, size, off})
		}
	}
	if len(entries) == 0 {
		return 0, errors.New("aucune image d'icône")
	}
	sort.Slice(entries, func(i, j int) bool {
		di := abs(entries[i].w-int(cx)) + abs(entries[i].h-int(cy))
		dj := abs(entries[j].w-int(cx)) + abs(entries[j].h-int(cy))
		return di < dj
	})
	e := entries[0]
	img := data[e.off : e.off+e.size]
	h, _, err := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&img[0])), uintptr(len(img)), 1, 0x00030000, uintptr(cx), uintptr(cy), LR_DEFAULTCOLOR)
	if h == 0 {
		return 0, err
	}
	return h, nil
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func addTrayIcon() {
	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = app.hwnd
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_APP_TRAY
	nid.HIcon = app.hIcon
	tip := syscall.StringToUTF16(appName + " " + appCredit)
	copy(nid.SzTip[:], tip)
	pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
}
func removeTrayIcon() {
	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = app.hwnd
	nid.UID = 1
	pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
}

func popupTrayMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	defer pDestroyMenu.Call(menu)
	appendMenu := func(flags uint32, id uintptr, text string) {
		var p uintptr
		if text != "" {
			p = uintptr(unsafe.Pointer(u16(text)))
		}
		pAppendMenuW.Call(menu, uintptr(flags), id, p)
	}
	appendMenu(MF_STRING, MENU_OPEN, "Ouvrir OtaWatt")
	appendMenu(MF_SEPARATOR, 0, "")

	app.mu.Lock()
	rec := app.recording
	app.mu.Unlock()
	if rec {
		appendMenu(MF_GRAYED|MF_STRING, MENU_RECORD, "Enregistrer")
		appendMenu(MF_STRING, MENU_STOP, "Arrêter l'enregistrement")
	} else {
		appendMenu(MF_STRING, MENU_RECORD, "Enregistrer")
		appendMenu(MF_GRAYED|MF_STRING, MENU_STOP, "Arrêter l'enregistrement")
	}
	flags := uint32(MF_STRING)
	if app.cfg.TopMost {
		flags |= MF_CHECKED
	}
	appendMenu(flags, MENU_TOPMOST, "Toujours au premier plan")
	appendMenu(MF_STRING, ID_COMPACT, "Mode compact 70 %")
	appendMenu(MF_STRING, MENU_SETTINGS, "Paramètres")
	appendMenu(MF_STRING, MENU_LOGS, "Ouvrir les registres")
	appendMenu(MF_SEPARATOR, 0, "")
	appendMenu(MF_STRING, MENU_EXIT, "Quitter complètement")

	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(app.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON|TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, app.hwnd, 0)
	if cmd != 0 {
		handleCommand(int(cmd))
	}
}

// --- Raccourcis Windows (.lnk) via COM, sans PowerShell ni registre de démarrage. ---
type iShellLinkW struct{ lpVtbl *[21]uintptr }
type iPersistFile struct{ lpVtbl *[9]uintptr }

var (
	clsidShellLink  = GUID{Data1: 0x00021401, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellLinkW  = GUID{Data1: 0x000214F9, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPersistFile = GUID{Data1: 0x0000010b, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

func hrFailed(hr uintptr) bool { return int32(hr) < 0 }

func createShortcut(linkPath, target, args, workDir, description, iconPath string) error {
	var sl *iShellLinkW
	hr, _, _ := pCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, 1,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&sl)),
	)
	if hrFailed(hr) || sl == nil {
		return fmt.Errorf("création du raccourci impossible")
	}
	defer syscall.SyscallN(sl.lpVtbl[2], uintptr(unsafe.Pointer(sl)))

	call := func(idx int, vals ...uintptr) error {
		a := []uintptr{uintptr(unsafe.Pointer(sl))}
		a = append(a, vals...)
		r, _, _ := syscall.SyscallN(sl.lpVtbl[idx], a...)
		if hrFailed(r) {
			return fmt.Errorf("erreur raccourci Windows")
		}
		return nil
	}
	if err := call(20, uintptr(unsafe.Pointer(u16(target)))); err != nil {
		return err
	}
	if args != "" {
		if err := call(11, uintptr(unsafe.Pointer(u16(args)))); err != nil {
			return err
		}
	}
	if workDir != "" {
		if err := call(9, uintptr(unsafe.Pointer(u16(workDir)))); err != nil {
			return err
		}
	}
	if description != "" {
		_ = call(7, uintptr(unsafe.Pointer(u16(description))))
	}
	if iconPath != "" {
		_ = call(17, uintptr(unsafe.Pointer(u16(iconPath))), 0)
	}

	var pf *iPersistFile
	r, _, _ := syscall.SyscallN(sl.lpVtbl[0], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&pf)))
	if hrFailed(r) || pf == nil {
		return fmt.Errorf("sauvegarde du raccourci impossible")
	}
	defer syscall.SyscallN(pf.lpVtbl[2], uintptr(unsafe.Pointer(pf)))
	_ = os.MkdirAll(filepath.Dir(linkPath), 0755)
	r, _, _ = syscall.SyscallN(pf.lpVtbl[6], uintptr(unsafe.Pointer(pf)), uintptr(unsafe.Pointer(u16(linkPath))), 1)
	if hrFailed(r) {
		return fmt.Errorf("écriture du raccourci impossible")
	}
	return nil
}

func startupShortcutPath() string {
	base := os.Getenv("APPDATA")
	return filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "OtaWatt.lnk")
}

// isPackaged indique si OtaWatt s'exécute depuis un package MSIX.
// GetCurrentPackageFullName renvoie APPMODEL_ERROR_NO_PACKAGE pour une version portable classique.
func isPackaged() bool {
	var n uint32
	r, _, _ := pGetCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0)
	code := uint32(r)
	return code == 0 || code == ERROR_INSUFFICIENT_BUFFER
}

func startupEnabled() bool {
	_, err := os.Stat(startupShortcutPath())
	return err == nil
}

// startupTarget choisit une cible stable. Dans le Store/MSIX, l'exécutable réel
// vit dans un dossier versionné WindowsApps. L'alias d'exécution déclaré dans le
// manifeste reste stable entre les mises à jour, ce qui évite un raccourci cassé.
func startupTarget() (target, workDir, iconPath string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", "", err
	}
	exe, _ = filepath.Abs(exe)
	if isPackaged() {
		base := os.Getenv("LOCALAPPDATA")
		if base != "" {
			alias := filepath.Join(base, "Microsoft", "WindowsApps", "OtaWatt.exe")
			return alias, filepath.Dir(alias), exe, nil
		}
	}
	return exe, filepath.Dir(exe), exe, nil
}

func setStartupEnabled(enableIt bool) error {
	link := startupShortcutPath()
	if !enableIt {
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	target, workDir, iconPath, err := startupTarget()
	if err != nil {
		return err
	}
	return createShortcut(link, target, "--startup", workDir, "OtaWatt - lancement automatique", iconPath)
}

func showWindowAnimated() {
	updateDataUI()
	r, _, _ := pAnimateWindow.Call(app.hwnd, FADE_MS, AW_ACTIVATE|AW_BLEND)
	if r == 0 {
		pShowWindow.Call(app.hwnd, SW_SHOW)
	}
	if app.compact {
		compactOpacity()
	} else {
		fullOpacity()
	}
	pSetForegroundWindow.Call(app.hwnd)
}

func hideWindowAnimated() {
	r, _, _ := pAnimateWindow.Call(app.hwnd, FADE_MS, AW_HIDE|AW_BLEND)
	if r == 0 {
		pShowWindow.Call(app.hwnd, SW_HIDE)
	}
}

func normalizeAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("adresse vide")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", errors.New("adresse invalide")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("protocole non pris en charge")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

func fetchStatus(ctx context.Context, address string) (*SwitchStatus, error) {
	base, err := normalizeAddress(address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rpc/Switch.GetStatus?id=0", nil)
	if err != nil {
		return nil, err
	}
	resp, err := app.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("réponse HTTP %d", resp.StatusCode)
	}
	var st SwitchStatus
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	if err := dec.Decode(&st); err != nil {
		return nil, err
	}
	if st.ID != 0 {
		return nil, errors.New("réponse Shelly inattendue")
	}
	return &st, nil
}

func testShelly(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := fetchStatus(ctx, address)
	return err
}

func configDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "OtaWatt")
}
func loadConfig() {
	app.configPath = filepath.Join(configDir(), "config.json")
	// Le comportement historique d'OtaWatt est de démarrer réduit lors d'un
	// lancement automatique. On conserve ce défaut pour les nouvelles
	// installations et les anciennes configurations qui ne possèdent pas
	// encore le champ start_minimized.
	app.cfg.StartMinimized = true
	b, err := os.ReadFile(app.configPath)
	if err != nil {
		return
	}
	var c Config
	if json.Unmarshal(b, &c) == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(b, &raw) == nil {
			if _, ok := raw["start_minimized"]; !ok {
				c.StartMinimized = true
			}
		}
		app.cfg = c
		if strings.TrimSpace(c.ShellyAddress) != "" {
			app.configured = true
		}
	}
}
func saveConfig() {
	_ = os.MkdirAll(filepath.Dir(app.configPath), 0755)
	b, _ := json.MarshalIndent(app.cfg, "", "  ")
	_ = os.WriteFile(app.configPath, b, 0644)
}

func documentsDir() string {
	// FOLDERID_Documents
	id := GUID{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	var path *uint16
	r, _, _ := pSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&path)))
	if r == 0 && path != nil {
		s := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(path))[:])
		pCoTaskMemFree.Call(uintptr(unsafe.Pointer(path)))
		if s != "" {
			return s
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

func openPath(path string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(path))), 0, 0, SW_SHOWNORMAL)
}

func makeLogFile() error {
	if err := os.MkdirAll(app.logsDir, 0755); err != nil {
		return err
	}
	name := "OtaWatt_" + time.Now().Format("2006-01-02_15-04-05") + ".csv"
	f, err := os.Create(filepath.Join(app.logsDir, name))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Comma = ';'
	if err := w.Write([]string{"Horodatage", "Puissance_W", "Tension_V", "Intensite_A", "Frequence_Hz", "Temperature_C", "Energie_Wh"}); err != nil {
		f.Close()
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	// Le fichier doit être réellement persistant dès le début. Flush vide le
	// tampon CSV vers os.File, puis Sync demande à Windows de pousser les
	// données vers le stockage. Cela évite de perdre le journal lors d'un BSOD.
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	app.recFile = f
	app.recCSV = w
	app.recStart = time.Now()
	app.recPending = 0
	app.recLastFlush = time.Now()
	return nil
}
func stopRecording(openFolder bool) {
	app.mu.Lock()
	if !app.recording {
		app.mu.Unlock()
		return
	}
	app.recording = false
	if app.recCSV != nil {
		app.recCSV.Flush()
	}
	if app.recFile != nil {
		_ = app.recFile.Close()
	}
	app.recCSV = nil
	app.recFile = nil
	app.mu.Unlock()
	updateRecordingControls()
	if openFolder {
		openPath(app.logsDir)
	}
}
func writeLog(st *SwitchStatus) {
	app.mu.Lock()
	defer app.mu.Unlock()
	if !app.recording || app.recCSV == nil {
		return
	}
	row := []string{
		time.Now().Format("2006-01-02 15:04:05.000"),
		strconv.FormatFloat(st.APower, 'f', 1, 64),
		strconv.FormatFloat(st.Voltage, 'f', 1, 64),
		strconv.FormatFloat(st.Current, 'f', 3, 64),
		strconv.FormatFloat(st.Freq, 'f', 1, 64),
		strconv.FormatFloat(st.Temperature.TC, 'f', 1, 64),
		strconv.FormatFloat(st.AEnergy.Total, 'f', 3, 64),
	}
	if err := app.recCSV.Write(row); err != nil {
		return
	}

	// Persistance anti-crash : chaque mesure (500 ms) est poussée jusqu'au fichier
	// puis synchronisée avec le stockage. On privilégie ici la conservation du
	// journal en cas de BSOD à un buffering de plusieurs secondes. Deux Sync/s
	// restent négligeables pour la charge CPU et le volume écrit par OtaWatt.
	app.recCSV.Flush()
	if err := app.recCSV.Error(); err != nil {
		return
	}
	if app.recFile != nil {
		if err := app.recFile.Sync(); err != nil {
			return
		}
	}
	app.recPending = 0
	app.recLastFlush = time.Now()
}

func startPolling() {
	if app.pollCancel != nil {
		app.pollCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.pollCancel = cancel

	go func() {
		const normalInterval = 500 * time.Millisecond
		const retryInterval = 2 * time.Second
		const requestTimeout = 1200 * time.Millisecond

		failures := 0
		delay := time.Duration(0)
		for {
			if delay > 0 {
				t := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					t.Stop()
					return
				case <-t.C:
				}
			}

			if !app.configured {
				delay = normalInterval
				continue
			}

			reqctx, c := context.WithTimeout(ctx, requestTimeout)
			st, err := fetchStatus(reqctx, app.cfg.ShellyAddress)
			c()

			app.mu.Lock()
			app.lastErr = err
			if err == nil {
				app.latest = st
			}
			app.mu.Unlock()

			if err == nil {
				failures = 0
				delay = normalInterval
				writeLog(st)
			} else {
				failures++
				if failures >= 3 {
					delay = retryInterval
				} else {
					delay = normalInterval
				}
			}

			// Quand la fenêtre est cachée, aucune mise à jour graphique n'est calculée.
			visible, _, _ := pIsWindowVisible.Call(app.hwnd)
			if visible != 0 {
				pPostMessageW.Call(app.hwnd, WM_APP_DATA, 0, 0)
			}
		}
	}()
}

func health(st *SwitchStatus) []string {
	set := map[string]bool{}
	add := func(s string) {
		if s != "" {
			set[s] = true
		}
	}
	for _, e := range st.Errors {
		switch strings.ToLower(strings.TrimSpace(e)) {
		case "overtemp", "over_temperature", "overtemperature":
			add("Surchauffe")
		case "overpower", "over_power":
			add("Surpuissance")
		case "overvoltage", "over_voltage":
			add("Surtension")
		case "undervoltage", "under_voltage":
			add("Sous-tension")
		case "overcurrent", "over_current":
			add("Surintensité")
		default:
			if strings.TrimSpace(e) != "" {
				add(strings.TrimSpace(e))
			}
		}
	}
	// Diagnostics OtaWatt, volontairement conservateurs.
	if st.Voltage > 0 && st.Voltage < 207 {
		add("Sous-tension")
	}
	if st.Voltage > 253 {
		add("Surtension")
	}
	if st.Current > 13.0 {
		add("Surintensité")
	}
	if st.APower > 3000 {
		add("Surpuissance")
	}
	if st.Freq > 0 && (st.Freq < 49.5 || st.Freq > 50.5) {
		add("Fréquence anormale")
	}
	if st.Temperature.TC >= 80 {
		add("Surchauffe")
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fr1(v float64) string { return strings.ReplaceAll(fmt.Sprintf("%.1f", v), ".", ",") }
func fr3(v float64) string { return strings.ReplaceAll(fmt.Sprintf("%.3f", v), ".", ",") }

func updateDataUI() {
	app.mu.Lock()
	st := app.latest
	err := app.lastErr
	rec := app.recording
	start := app.recStart
	app.mu.Unlock()
	if err != nil {
		setText(app.controls[ID_STATUS], "Shelly inaccessible - nouvelle tentative automatique")
		app.staticColor[app.controls[ID_STATUS]] = colOrange
		return
	}
	if st == nil {
		return
	}
	setText(app.controls[ID_POWER], fr1(st.APower)+" W")
	setText(app.controls[ID_VOLTAGE], "Tension : "+fr1(st.Voltage)+" V")
	setText(app.controls[ID_CURRENT], "Intensité : "+fr3(st.Current)+" A")
	setText(app.controls[ID_FREQ], "Fréquence : "+fr1(st.Freq)+" Hz")
	setText(app.controls[ID_TEMP], "Température : "+fr1(st.Temperature.TC)+" °C")

	setText(app.controls[ID_C_POWER], fr1(st.APower)+" W")
	setText(app.controls[ID_C_VOLTAGE], "Tension : "+fr1(st.Voltage)+" V")
	setText(app.controls[ID_C_CURRENT], "Intensité : "+fr3(st.Current)+" A")
	setText(app.controls[ID_C_TEMP], "Température : "+fr1(st.Temperature.TC)+" °C")

	alerts := health(st)
	if len(alerts) == 0 {
		setText(app.controls[ID_HEALTH], "État de santé : RAS")
		setText(app.controls[ID_C_HEALTH], "État de santé : RAS")
		app.staticColor[app.controls[ID_HEALTH]] = colGreen
		app.staticColor[app.controls[ID_C_HEALTH]] = colGreen
	} else {
		healthText := "État de santé :\r\n" + strings.Join(alerts, "\r\n")
		setText(app.controls[ID_HEALTH], healthText)
		setText(app.controls[ID_C_HEALTH], healthText)
		app.staticColor[app.controls[ID_HEALTH]] = colRed
		app.staticColor[app.controls[ID_C_HEALTH]] = colRed
	}
	if rec {
		d := time.Since(start).Truncate(time.Second)
		setText(app.controls[ID_STATUS], "Enregistrement : "+formatDuration(d))
		app.staticColor[app.controls[ID_STATUS]] = colMagenta
	} else {
		setText(app.controls[ID_STATUS], "Connecté")
		app.staticColor[app.controls[ID_STATUS]] = colGreen
	}
}
func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func updateRecordingControls() {
	app.mu.Lock()
	rec := app.recording
	app.mu.Unlock()
	enable(app.controls[ID_START], !rec)
	enable(app.controls[ID_STOP], rec)
}

func applyTopMost() {
	target := HWND_NOTOPMOST
	if app.cfg.TopMost {
		target = HWND_TOPMOST
	}
	pSetWindowPos.Call(app.hwnd, target, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE)
	if h := app.controls[ID_SET_TOPMOST]; h != 0 {
		setChecked(h, app.cfg.TopMost)
	}
}

func allModeIDs() (mainIDs, compactIDs, cfgIDs, setIDs []int) {
	mainIDs = []int{ID_POWER, ID_VOLTAGE, ID_CURRENT, ID_FREQ, ID_TEMP, ID_HEALTH, ID_STATUS, ID_START, ID_STOP, ID_SETTINGS, ID_COMPACT}
	compactIDs = []int{ID_C_POWER, ID_C_VOLTAGE, ID_C_CURRENT, ID_C_TEMP, ID_C_HEALTH, ID_C_BACK}
	cfgIDs = []int{ID_CFG_INFO, ID_CFG_LABEL, ID_CFG_EDIT, ID_CFG_TEST, ID_CFG_STATUS, ID_CFG_CONTINUE, ID_CFG_BACK}
	setIDs = []int{ID_SET_INFO, ID_SET_SEPARATOR, ID_SET_TOPMOST, ID_SET_STARTUP, ID_SET_STARTMIN, ID_SET_STARTUP_NOTE, ID_SET_SHELLY, ID_SET_LOGS, ID_SET_BACK, ID_SET_STATUS}
	return
}

func hideIDs(ids []int) {
	for _, id := range ids {
		if h := app.controls[id]; h != 0 {
			show(h, false)
		}
	}
}

func showIDs(ids []int) {
	for _, id := range ids {
		if h := app.controls[id]; h != 0 {
			show(h, true)
		}
	}
}

func showMain() {
	app.showingConfig = false
	app.showingOptions = false
	app.compact = false
	setWindowSize(FULL_WIDTH, FULL_HEIGHT)
	fullOpacity()

	show(app.controls[ID_ICON], true)
	show(app.controls[ID_TITLE], true)
	show(app.controls[ID_CREDIT], true)
	mainIDs, compactIDs, cfgIDs, setIDs := allModeIDs()
	hideIDs(append(append(compactIDs, cfgIDs...), setIDs...))
	showIDs(mainIDs)
	updateRecordingControls()
	applyTopMost()
	updateDataUI()
	startPolling()
}

func showCompact() {
	app.showingConfig = false
	app.showingOptions = false
	app.compact = true
	setWindowSize(COMPACT_WIDTH, COMPACT_HEIGHT)
	compactOpacity()

	show(app.controls[ID_ICON], false)
	show(app.controls[ID_TITLE], false)
	show(app.controls[ID_CREDIT], false)
	mainIDs, compactIDs, cfgIDs, setIDs := allModeIDs()
	hideIDs(append(append(mainIDs, cfgIDs...), setIDs...))
	showIDs(compactIDs)
	applyTopMost()
	updateDataUI()
	startPolling()
}

func showConfig() {
	app.showingConfig = true
	app.showingOptions = false
	app.compact = false
	setWindowSize(FULL_WIDTH, FULL_HEIGHT)
	fullOpacity()

	show(app.controls[ID_ICON], true)
	show(app.controls[ID_TITLE], true)
	show(app.controls[ID_CREDIT], true)
	mainIDs, compactIDs, cfgIDs, setIDs := allModeIDs()
	hideIDs(append(append(mainIDs, compactIDs...), setIDs...))
	showIDs(cfgIDs)
	setText(app.controls[ID_CFG_EDIT], app.cfg.ShellyAddress)
	setText(app.controls[ID_CFG_STATUS], "Saisis l'adresse IP puis teste la connexion.")
	app.staticColor[app.controls[ID_CFG_STATUS]] = colDim
	app.testedAddress = ""
	enable(app.controls[ID_CFG_CONTINUE], false)
	show(app.controls[ID_CFG_BACK], app.configured)
}

func syncSettingsChecks() {
	setChecked(app.controls[ID_SET_TOPMOST], app.cfg.TopMost)
	setChecked(app.controls[ID_SET_STARTUP], startupEnabled())
	setChecked(app.controls[ID_SET_STARTMIN], app.cfg.StartMinimized)
}

func showSettings() {
	app.showingConfig = false
	app.showingOptions = true
	app.compact = false
	setWindowSize(FULL_WIDTH, FULL_HEIGHT)
	fullOpacity()

	show(app.controls[ID_ICON], true)
	show(app.controls[ID_TITLE], true)
	show(app.controls[ID_CREDIT], true)
	mainIDs, compactIDs, cfgIDs, setIDs := allModeIDs()
	hideIDs(append(append(mainIDs, compactIDs...), cfgIDs...))
	showIDs(setIDs)
	syncSettingsChecks()
	setText(app.controls[ID_SET_STATUS], "")
	applyTopMost()
}

func handleCommand(id int) {
	switch id {
	case ID_START, MENU_RECORD:
		app.mu.Lock()
		if !app.recording {
			if err := makeLogFile(); err != nil {
				app.mu.Unlock()
				setText(app.controls[ID_STATUS], "Impossible de créer le registre CSV")
				return
			}
			app.recording = true
		}
		app.mu.Unlock()
		updateRecordingControls()
		updateDataUI()
	case ID_STOP, MENU_STOP:
		stopRecording(true)
		updateDataUI()
	case MENU_TOPMOST:
		app.cfg.TopMost = !app.cfg.TopMost
		saveConfig()
		applyTopMost()
	case ID_COMPACT:
		showCompact()
	case ID_C_BACK:
		showMain()
	case ID_SETTINGS, MENU_SETTINGS:
		showSettings()
		showWindowAnimated()
	case ID_SET_TOPMOST:
		app.cfg.TopMost = isChecked(app.controls[ID_SET_TOPMOST])
		saveConfig()
		applyTopMost()
	case ID_SET_STARTUP:
		wanted := isChecked(app.controls[ID_SET_STARTUP])
		if err := setStartupEnabled(wanted); err != nil {
			setChecked(app.controls[ID_SET_STARTUP], !wanted)
			setText(app.controls[ID_SET_STATUS], "Impossible de modifier le démarrage avec Windows.")
			app.staticColor[app.controls[ID_SET_STATUS]] = colRed
		} else {
			setText(app.controls[ID_SET_STATUS], "Paramètre de démarrage Windows enregistré.")
			app.staticColor[app.controls[ID_SET_STATUS]] = colGreen
		}
	case ID_SET_STARTMIN:
		app.cfg.StartMinimized = isChecked(app.controls[ID_SET_STARTMIN])
		saveConfig()
		setText(app.controls[ID_SET_STATUS], "Mode de démarrage enregistré.")
		app.staticColor[app.controls[ID_SET_STATUS]] = colGreen
	case ID_SET_SHELLY:
		app.configReturnSettings = true
		showConfig()
	case ID_SET_LOGS, MENU_LOGS:
		_ = os.MkdirAll(app.logsDir, 0755)
		openPath(app.logsDir)
	case ID_SET_BACK:
		showMain()
	case ID_CFG_TEST:
		address := strings.TrimSpace(getText(app.controls[ID_CFG_EDIT]))
		setText(app.controls[ID_CFG_STATUS], "Test de connexion...")
		app.staticColor[app.controls[ID_CFG_STATUS]] = colMagenta
		enable(app.controls[ID_CFG_TEST], false)
		go func(a string) {
			err := testShelly(a)
			app.mu.Lock()
			app.lastErr = err
			if err == nil {
				app.testedAddress = a
			} else {
				app.testedAddress = ""
			}
			app.mu.Unlock()
			pPostMessageW.Call(app.hwnd, WM_APP_DATA, 1, 0)
		}(address)
	case ID_CFG_CONTINUE:
		address := strings.TrimSpace(getText(app.controls[ID_CFG_EDIT]))
		app.mu.Lock()
		testedAddress := app.testedAddress
		app.mu.Unlock()
		if testedAddress == "" || testedAddress != address {
			setText(app.controls[ID_CFG_STATUS], "Teste d'abord cette adresse Shelly.")
			app.staticColor[app.controls[ID_CFG_STATUS]] = colOrange
			return
		}
		app.cfg.ShellyAddress = address
		app.configured = true
		saveConfig()
		if app.configReturnSettings {
			app.configReturnSettings = false
			showSettings()
		} else {
			showMain()
		}
	case ID_CFG_BACK:
		if app.configured {
			if app.configReturnSettings {
				app.configReturnSettings = false
				showSettings()
			} else {
				showMain()
			}
		}
	case MENU_OPEN:
		if app.showingConfig {
			showConfig()
		} else if app.showingOptions {
			showSettings()
		} else if app.compact {
			showCompact()
		} else {
			showMain()
		}
		showWindowAnimated()
	case MENU_EXIT:
		app.quitting = true
		hideWindowAnimated()
		pDestroyWindow.Call(app.hwnd)
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		handleCommand(int(loword(wParam)))
		return 0
	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if dis != nil {
			drawOwnerButton(dis)
			return 1
		}
	case WM_CTLCOLORSTATIC:
		hdc := wParam
		ctrl := lParam
		if ctrl == app.controls[ID_SET_SEPARATOR] && app.separatorBrush != 0 {
			return app.separatorBrush
		}
		color := colWhite
		if c, ok := app.staticColor[ctrl]; ok {
			color = c
		}
		pSetTextColor.Call(hdc, uintptr(color))
		pSetBkMode.Call(hdc, TRANSPARENT)
		return app.bgBrush
	case WM_CTLCOLOREDIT:
		hdc := wParam
		pSetTextColor.Call(hdc, uintptr(colWhite))
		pSetBkColor.Call(hdc, uintptr(colPanel))
		return app.panelBrush
	case WM_CTLCOLORBTN:
		hdc := wParam
		pSetTextColor.Call(hdc, uintptr(colWhite))
		pSetBkMode.Call(hdc, TRANSPARENT)
		return app.bgBrush
	case WM_APP_DATA:
		if wParam == 1 {
			enable(app.controls[ID_CFG_TEST], true)
			app.mu.Lock()
			err := app.lastErr
			testedAddress := app.testedAddress
			app.mu.Unlock()
			if err == nil && testedAddress != "" {
				setText(app.controls[ID_CFG_STATUS], "Connexion réussie - Shelly détecté")
				app.staticColor[app.controls[ID_CFG_STATUS]] = colGreen
				enable(app.controls[ID_CFG_CONTINUE], true)
			} else {
				setText(app.controls[ID_CFG_STATUS], "Connexion impossible - vérifie l'adresse IP")
				app.staticColor[app.controls[ID_CFG_STATUS]] = colRed
				enable(app.controls[ID_CFG_CONTINUE], false)
			}
		} else {
			updateDataUI()
		}
		return 0
	case WM_APP_TRAY:
		switch uint32(lParam) {
		case WM_LBUTTONDBLCLK:
			showWindowAnimated()
		case WM_RBUTTONUP:
			popupTrayMenu()
		}
		return 0
	case WM_CLOSE:
		if app.quitting {
			pDestroyWindow.Call(hwnd)
		} else {
			hideWindowAnimated()
		}
		return 0
	case WM_DESTROY:
		if app.pollCancel != nil {
			app.pollCancel()
		}
		stopRecording(false)
		removeTrayIcon()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func buildUI() error {
	app.controls = map[int]uintptr{}
	app.buttonAccent = map[uintptr]uint32{}
	app.staticColor = map[uintptr]uint32{}
	app.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(colBG))
	app.panelBrush, _, _ = pCreateSolidBrush.Call(uintptr(colPanel))
	app.pressedBrush, _, _ = pCreateSolidBrush.Call(uintptr(colPressed))
	app.separatorBrush, _, _ = pCreateSolidBrush.Call(uintptr(colGreen))
	fontFace := "Segoe UI"
	if loadEmbeddedAudiowide() {
		fontFace = "Audiowide"
	}
	app.fontTitle = createFontFace(-32, 700, fontFace)
	app.fontCredit = createFontFace(-15, 400, fontFace)
	app.fontPower = createFontFace(-48, 700, fontFace)
	app.fontMeasure = createFontFace(-20, 500, fontFace)
	app.fontHealth = createFontFace(-18, 600, fontFace)
	app.fontNormal = createFontFace(-17, 500, fontFace)
	app.fontSmall = createFontFace(-14, 400, fontFace)
	app.fontButton = createFontFace(-17, 500, "Segoe UI")
	app.fontButtonSmall = createFontFace(-12, 500, "Segoe UI")
	app.fontEdit = createFontFace(-17, 500, "Segoe UI")

	icoBytes, err := embeddedFS.ReadFile("OtaWatt.ico")
	if err == nil {
		app.hIcon, _ = iconFromICO(icoBytes, 64, 64)
	}

	hInst, _, _ := pGetModuleHandleW.Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	className := u16("OtaWattMainWindow")
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hInst, HIcon: app.hIcon, HCursor: cursor, HbrBackground: app.bgBrush, LpszClassName: className, HIconSm: app.hIcon}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}

	width, height := int32(FULL_WIDTH), int32(FULL_HEIGHT)
	sx, _, _ := pGetSystemMetrics.Call(0)
	sy, _, _ := pGetSystemMetrics.Call(1)
	x := (int32(sx) - width) / 2
	y := (int32(sy) - height) / 2
	style := uint32(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)
	hwnd, _, err := pCreateWindowExW.Call(WS_EX_APPWINDOW|WS_EX_LAYERED, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(u16(appName+" "+appCredit))), uintptr(style), uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, hInst, 0)
	if hwnd == 0 {
		return err
	}
	app.hwnd = hwnd
	pSendMessageW.Call(hwnd, WM_SETICON, 1, app.hIcon)
	pSendMessageW.Call(hwnd, WM_SETICON, 0, app.hIcon)

	// Header
	iconWnd := createControl("STATIC", "", WS_CHILD|WS_VISIBLE|SS_ICON|SS_CENTERIMAGE, 0, 28, 20, 72, 72, ID_ICON)
	pSendMessageW.Call(iconWnd, STM_SETICON, app.hIcon, 0)
	addStatic(ID_TITLE, "OtaWatt", 105, 22, 340, 42, app.fontTitle, colViolet)
	addStatic(ID_CREDIT, appCredit, 105, 62, 340, 24, app.fontCredit, colGreen)

	// Main screen
	addStatic(ID_POWER, "--- W", 20, 105, 450, 65, app.fontPower, colWhite)
	addStatic(ID_VOLTAGE, "Tension : --- V", 20, 185, 450, 28, app.fontMeasure, colDim)
	addStatic(ID_CURRENT, "Intensité : --- A", 20, 217, 450, 28, app.fontMeasure, colDim)
	addStatic(ID_FREQ, "Fréquence : --- Hz", 20, 249, 450, 28, app.fontMeasure, colDim)
	addStatic(ID_TEMP, "Température : --- °C", 20, 281, 450, 28, app.fontMeasure, colDim)
	addStatic(ID_HEALTH, "État de santé : RAS", 35, 325, 420, 80, app.fontHealth, colGreen)
	addStatic(ID_STATUS, "Connexion...", 30, 408, 430, 25, app.fontSmall, colMagenta)
	addButton(ID_START, "Enregistrer", 45, 455, 190, 44, colGreen)
	addButton(ID_STOP, "Arrêter", 265, 455, 190, 44, colMagenta)
	addButton(ID_SETTINGS, "Paramètres", 45, 525, 190, 40, colViolet)
	addButton(ID_COMPACT, "Mode compact 70 %", 265, 525, 190, 40, colMagenta)

	// Mode compact : marges minimales, groupe vertical resserré, opacité 70 %.
	addStatic(ID_C_POWER, "--- W", 2, 8, 316, 54, app.fontPower, colWhite)
	addStatic(ID_C_VOLTAGE, "Tension : --- V", 2, 66, 316, 28, app.fontMeasure, colDim)
	addStatic(ID_C_CURRENT, "Intensité : --- A", 2, 94, 316, 28, app.fontMeasure, colDim)
	addStatic(ID_C_TEMP, "Température : --- °C", 2, 122, 316, 28, app.fontMeasure, colDim)
	addStatic(ID_C_HEALTH, "État de santé : RAS", 4, 157, 312, 50, app.fontHealth, colGreen)
	compactBack := addButton(ID_C_BACK, "Revenir en format complet", 60, 210, 200, 27, colViolet)
	setFont(compactBack, app.fontButtonSmall)

	// Configuration screen
	addStatic(ID_CFG_INFO, "Configuration du Shelly", 30, 120, 440, 34, app.fontMeasure, colWhite)
	addStatic(ID_CFG_LABEL, "Adresse IP ou nom réseau du Shelly", 30, 175, 440, 28, app.fontNormal, colDim)
	addEdit(ID_CFG_EDIT, "", 90, 215, 320, 38)
	addButton(ID_CFG_TEST, "Tester la connexion", 130, 270, 240, 44, colGreen)
	addStatic(ID_CFG_STATUS, "Saisis l'adresse IP puis teste la connexion.", 30, 330, 440, 60, app.fontSmall, colDim)
	addButton(ID_CFG_CONTINUE, "Continuer", 140, 415, 220, 44, colViolet)
	addButton(ID_CFG_BACK, "Retour", 170, 475, 160, 40, colMagenta)

	// Paramètres centralisés : trois options indépendantes, puis outils.
	addStatic(ID_SET_INFO, "Paramètres", 30, 120, 440, 34, app.fontMeasure, colWhite)
	createControl("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 70, 162, 360, 2, ID_SET_SEPARATOR)
	addCheckbox(ID_SET_TOPMOST, "Toujours au premier plan", 70, 184, 370, 30)
	addCheckboxMultiline(ID_SET_STARTUP, "Lancer OtaWatt au démarrage de\r\nWindows", 55, 222, 400, 50)
	addCheckbox(ID_SET_STARTMIN, "Démarrer en mode réduit", 70, 278, 370, 30)
	addStatic(ID_SET_STARTUP_NOTE, "Si activé, OtaWatt démarre en mode réduit dans la zone de notification.", 50, 311, 400, 42, app.fontSmall, colDim)
	addButton(ID_SET_SHELLY, "Configurer le Shelly", 80, 370, 340, 42, colGreen)
	addButton(ID_SET_LOGS, "Ouvrir les registres", 80, 423, 340, 42, colViolet)
	addStatic(ID_SET_STATUS, "", 45, 475, 410, 42, app.fontSmall, colDim)
	addButton(ID_SET_BACK, "Retour", 170, 530, 160, 40, colMagenta)

	addTrayIcon()
	if app.configured {
		showMain()
	} else {
		app.configReturnSettings = false
		showConfig()
	}
	if startupLaunch && app.configured && app.cfg.StartMinimized {
		pShowWindow.Call(hwnd, SW_HIDE)
	} else {
		showWindowAnimated()
	}
	pUpdateWindow.Call(hwnd)
	return nil
}

func cleanup() {
	if app.pollCancel != nil {
		app.pollCancel()
	}
	if app.httpClient != nil {
		app.httpClient.CloseIdleConnections()
	}
	stopRecording(false)
	removeTrayIcon()
	for _, h := range []uintptr{app.bgBrush, app.panelBrush, app.pressedBrush, app.separatorBrush, app.fontTitle, app.fontCredit, app.fontPower, app.fontMeasure, app.fontHealth, app.fontNormal, app.fontSmall, app.fontButton, app.fontButtonSmall, app.fontEdit} {
		if h != 0 {
			pDeleteObject.Call(h)
		}
	}
	if app.fontMemHandle != 0 {
		pRemoveFontMemResourceEx.Call(app.fontMemHandle)
		app.fontMemHandle = 0
		app.fontMemData = nil
	}
	if app.hIcon != 0 {
		pDestroyIcon.Call(app.hIcon)
	}
}

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()
	for _, a := range os.Args[1:] {
		if strings.EqualFold(a, "--startup") || strings.EqualFold(a, "--tray") {
			startupLaunch = true
		}
	}
	// STA COM pour la création de raccourcis Windows.
	hr, _, _ := pCoInitializeEx.Call(0, 2)
	if !hrFailed(hr) {
		defer pCoUninitialize.Call()
	}

	mutexName := u16("Local\\OtaWatt_Application_Mutex")
	hMutex, _, mutexErr := pCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(mutexName)))
	if hMutex != 0 {
		defer pCloseHandle.Call(hMutex)
	}
	if errno, ok := mutexErr.(syscall.Errno); ok && uint32(errno) == ERROR_ALREADY_EXISTS {
		// Pas de sous-processus ni de mécanisme caché : deuxième instance simplement ignorée.
		return
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        2,
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     1,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
	app.httpClient = &http.Client{Transport: transport, Timeout: 1500 * time.Millisecond}
	loadConfig()
	app.logsDir = filepath.Join(documentsDir(), "OtaWatt", "Registres")

	if err := buildUI(); err != nil {
		return
	}
	defer cleanup()

	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

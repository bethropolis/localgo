package handlers

import (
	"crypto/subtle"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/model"
)

const webShareHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="dark">
    <meta name="theme-color" content="#04100a">
    <title>{{if .PinLocked}}Unlock · {{end}}LocalGo Share</title>
    <style>
        :root {
            --bg0: #04100a;
            --bg1: #071a10;
            --card: rgba(12, 28, 20, 0.72);
            --card-solid: #10261a;
            --border: rgba(134, 239, 172, 0.12);
            --border-strong: rgba(74, 222, 128, 0.32);
            --text: #eafdf3;
            --muted: #9fc2ae;
            --faint: #6b8f7a;
            --accent: #4ade80;
            --accent-2: #2dd4bf;
            --btn: #22c55e;
            --btn-hover: #16a34a;
            --btn-text: #052012;
            --danger: #fb7185;
            --danger-bg: rgba(251, 113, 133, 0.1);
            --ok: #4ade80;
            --shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.6);
            --radius: 18px;
            --radius-sm: 12px;
            --font: 'JetBrains Mono', 'Fira Code', ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
        }
        * { box-sizing: border-box; }
        html, body { min-height: 100%; }
        body {
            margin: 0;
            font-family: var(--font);
            color: var(--text);
            background:
                radial-gradient(1200px 600px at 10% -10%, rgba(74, 222, 128, 0.16), transparent 55%),
                radial-gradient(900px 500px at 100% 0%, rgba(45, 212, 191, 0.14), transparent 50%),
                radial-gradient(700px 400px at 50% 110%, rgba(34, 197, 94, 0.1), transparent 45%),
                linear-gradient(180deg, var(--bg0), var(--bg1) 55%, #051209);
            display: flex;
            justify-content: center;
            align-items: center;
            padding: 1.25rem;
            line-height: 1.5;
            -webkit-font-smoothing: antialiased;
        }
        .shell {
            width: 100%;
            max-width: 520px;
            animation: rise 0.45s ease-out;
        }
        @keyframes rise {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: none; }
        }
        .card {
            background: var(--card);
            backdrop-filter: blur(18px);
            -webkit-backdrop-filter: blur(18px);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            box-shadow: var(--shadow);
            overflow: hidden;
        }
        .card-head {
            padding: 1.4rem 1.5rem 1.25rem;
            border-bottom: 1px solid var(--border);
            background: linear-gradient(180deg, rgba(74, 222, 128, 0.08), transparent 80%);
        }
        .brand-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 0.75rem;
        }
        .brand-row h1 {
            margin: 0;
            font-size: 1.05rem;
            font-weight: 700;
            letter-spacing: -0.01em;
        }
        .brand-sub {
            margin: 0.3rem 0 0;
            color: var(--muted);
            font-size: 0.82rem;
        }
        .brand-sub strong { color: var(--text); font-weight: 600; }
        .stats {
            display: flex;
            flex-wrap: wrap;
            justify-content: center;
            gap: 0.5rem;
            margin-top: 1rem;
        }
        .chip {
            display: inline-flex;
            align-items: center;
            gap: 0.35rem;
            padding: 0.3rem 0.65rem;
            border-radius: 999px;
            font-size: 0.72rem;
            font-weight: 600;
            color: var(--muted);
            background: rgba(7, 20, 13, 0.55);
            border: 1px solid var(--border);
        }
        .chip svg { width: 12px; height: 12px; opacity: 0.9; flex-shrink: 0; }
        .card-body { padding: 1.1rem 1.25rem 1.35rem; }
        .file-list {
            list-style: none;
            margin: 0;
            padding: 0;
            display: flex;
            flex-direction: column;
            gap: 0.7rem;
        }
        .file-item {
            display: flex;
            align-items: center;
            gap: 0.85rem;
            padding: 0.85rem;
            border-radius: var(--radius-sm);
            background: rgba(7, 20, 13, 0.45);
            border: 1px solid var(--border);
            transition: border-color 0.15s ease, transform 0.15s ease, background 0.15s ease;
        }
        .file-item:hover {
            border-color: var(--border-strong);
            background: rgba(7, 20, 13, 0.72);
            transform: translateY(-1px);
        }
        .file-icon {
            width: 42px;
            height: 42px;
            border-radius: 11px;
            display: grid;
            place-items: center;
            flex-shrink: 0;
            background: rgba(74, 222, 128, 0.12);
            color: var(--accent);
            border: 1px solid rgba(74, 222, 128, 0.18);
        }
        .file-icon.image { background: rgba(45, 212, 191, 0.12); color: var(--accent-2); border-color: rgba(45, 212, 191, 0.22); }
        .file-icon.video { background: rgba(251, 113, 133, 0.12); color: var(--danger); border-color: rgba(251, 113, 133, 0.2); }
        .file-icon.audio { background: rgba(167, 139, 250, 0.12); color: #a78bfa; border-color: rgba(167, 139, 250, 0.2); }
        .file-icon.archive { background: rgba(251, 191, 36, 0.12); color: #fbbf24; border-color: rgba(251, 191, 36, 0.2); }
        .file-icon.code { background: rgba(129, 140, 248, 0.12); color: #818cf8; border-color: rgba(129, 140, 248, 0.2); }
        .file-icon svg { width: 20px; height: 20px; }
        .file-meta { min-width: 0; flex: 1; }
        .file-name {
            font-weight: 600;
            font-size: 0.9rem;
            word-break: break-word;
            letter-spacing: -0.005em;
        }
        .file-sub {
            display: flex;
            flex-wrap: wrap;
            align-items: center;
            gap: 0.4rem;
            margin-top: 0.3rem;
            color: var(--faint);
            font-size: 0.74rem;
        }
        .tag {
            padding: 0.05rem 0.4rem;
            border-radius: 5px;
            border: 1px solid var(--border);
            color: var(--muted);
            font-size: 0.68rem;
            letter-spacing: 0.03em;
        }
        .btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 0.4rem;
            background: linear-gradient(180deg, #4ade80, var(--btn));
            color: var(--btn-text);
            text-decoration: none;
            padding: 0.6rem 0.9rem;
            border-radius: 10px;
            font-weight: 700;
            font-size: 0.8rem;
            font-family: inherit;
            border: none;
            cursor: pointer;
            white-space: nowrap;
            box-shadow: 0 8px 18px -10px rgba(34, 197, 94, 0.9);
            transition: transform 0.12s ease, filter 0.12s ease, box-shadow 0.12s ease;
        }
        .btn:hover {
            filter: brightness(1.06);
            transform: translateY(-1px);
            box-shadow: 0 12px 22px -10px rgba(34, 197, 94, 1);
        }
        .btn:active { transform: translateY(0); }
        .btn:focus-visible, .pin-input:focus-visible {
            outline: 2px solid var(--accent);
            outline-offset: 2px;
        }
        .btn svg { width: 15px; height: 15px; flex-shrink: 0; }
        .pin-wrap {
            text-align: center;
            padding: 0.75rem 0.35rem 0.35rem;
        }
        .lock {
            width: 64px;
            height: 64px;
            margin: 0 auto 1rem;
            border-radius: 20px;
            display: grid;
            place-items: center;
            background: rgba(74, 222, 128, 0.1);
            border: 1px solid var(--border-strong);
            color: var(--accent);
        }
        .lock svg { width: 28px; height: 28px; }
        .pin-wrap h2 {
            margin: 0 0 0.4rem;
            font-size: 1.1rem;
            letter-spacing: -0.01em;
        }
        .pin-wrap p {
            margin: 0 auto 1.15rem;
            max-width: 28ch;
            color: var(--muted);
            font-size: 0.85rem;
        }
        .pin-form {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
            max-width: 280px;
            margin: 0 auto;
        }
        .pin-input {
            width: 100%;
            padding: 0.8rem 0.95rem;
            border-radius: 12px;
            border: 1px solid var(--border);
            background: rgba(7, 20, 13, 0.75);
            color: var(--text);
            font-family: inherit;
            font-size: 1rem;
            letter-spacing: 0.18em;
            text-align: center;
        }
        .pin-input::placeholder {
            letter-spacing: normal;
            color: var(--faint);
        }
        .pin-error {
            margin: 0 0 0.85rem;
            padding: 0.55rem 0.75rem;
            border-radius: 10px;
            background: var(--danger-bg);
            border: 1px solid rgba(251, 113, 133, 0.28);
            color: #fecdd3;
            font-size: 0.8rem;
        }
        .empty {
            text-align: center;
            color: var(--muted);
            padding: 1.5rem 0.5rem;
            font-size: 0.85rem;
        }
        .card-foot {
            padding: 0.85rem 1.25rem 1.1rem;
            border-top: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 0.75rem;
            color: var(--faint);
            font-size: 0.7rem;
        }
        .card-foot span { display: inline-flex; align-items: center; gap: 0.32rem; }
        .card-foot svg { width: 12px; height: 12px; }
        @media (max-width: 480px) {
            body { padding: 0.85rem; align-items: flex-start; padding-top: 1.5rem; }
            .file-item { flex-wrap: wrap; }
            .btn { width: 100%; }
            .file-meta { width: calc(100% - 50px); }
        }
        @media (prefers-reduced-motion: reduce) {
            .shell, .file-item, .btn { animation: none; transition: none; }
        }
    </style>
</head>
<body>
    <div class="shell">
        <div class="card">
            <div class="card-head">
                <h1>LocalGo Share</h1>
                <p class="brand-sub">Direct from <strong>{{.Alias}}</strong> &mdash; same network, no upload.</p>
                {{if not .PinLocked}}
                <div class="stats">
                    <span class="chip">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
                        {{.FileCount}} {{if eq .FileCount 1}}file{{else}}files{{end}}
                    </span>
                    <span class="chip">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></svg>
                        {{formatBytes .TotalSize}}
                    </span>
                </div>
                {{end}}
            </div>

            <div class="card-body">
                {{if .PinLocked}}
                <div class="pin-wrap">
                    <div class="lock" aria-hidden="true">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                            <rect x="4" y="11" width="16" height="10" rx="2"/>
                            <path d="M8 11V8a4 4 0 0 1 8 0v3"/>
                        </svg>
                    </div>
                    <h2>PIN protected</h2>
                    <p>Enter the PIN from the sender to view and download shared files.</p>
                    {{if .PinError}}
                    <div class="pin-error" role="alert">Incorrect PIN. Please try again.</div>
                    {{end}}
                    <form method="GET" action="/" class="pin-form">
                        <input type="password" name="pin" inputmode="numeric" autocomplete="one-time-code" placeholder="Enter PIN" class="pin-input" required autofocus aria-label="PIN">
                        <button type="submit" class="btn">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/></svg>
                            Unlock
                        </button>
                    </form>
                </div>
                {{else if eq .FileCount 0}}
                <div class="empty">No files are currently shared.</div>
                {{else}}
                <ul class="file-list" aria-label="Shared files">
                    {{range .Files}}
                    <li class="file-item">
                        <div class="file-icon {{.Kind}}" aria-hidden="true">
                            {{if eq .Kind "image"}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-5-5L5 21"/></svg>
                            {{else if eq .Kind "video"}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="14" height="14" rx="2"/><path d="m17 10 4-2v8l-4-2z"/></svg>
                            {{else if eq .Kind "audio"}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
                            {{else if eq .Kind "archive"}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
                            {{else if eq .Kind "code"}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m16 18 6-6-6-6"/><path d="m8 6-6 6 6 6"/></svg>
                            {{else}}
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
                            {{end}}
                        </div>
                        <div class="file-meta">
                            <div class="file-name">{{.FileName}}</div>
                            <div class="file-sub">
                                <span>{{formatBytes .Size}}</span>
                                {{if .Ext}}<span class="tag">{{.Ext}}</span>{{end}}
                            </div>
                        </div>
                        <a class="btn" href="/api/localsend/v2/download?sessionId={{$.SessionID}}&fileId={{.ID}}{{if $.PIN}}&pin={{$.PIN}}{{end}}" download="{{.FileName}}">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 19h14"/></svg>
                            Download
                        </a>
                    </li>
                    {{end}}
                </ul>
                {{end}}
            </div>

            <div class="card-foot">
                <span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 8.5a15 15 0 0 1 20 0"/><path d="M5 12a10 10 0 0 1 14 0"/><path d="M8.5 15.5a5 5 0 0 1 7 0"/><circle cx="12" cy="19" r="1" fill="currentColor" stroke="none"/></svg>
                    Local network transfer
                </span>
                <span>Powered by LocalGo</span>
            </div>
        </div>
    </div>
</body>
</html>`

// WebShareFile is one entry on the landing page file list.
type WebShareFile struct {
	ID       string
	FileName string
	Size     int64
	Ext      string
	Kind     string
}

// WebShareData holds template data for the web landing page.
type WebShareData struct {
	Alias     string
	SessionID string
	Files     []WebShareFile
	FileCount int
	TotalSize int64
	PinLocked bool
	PinError  bool
	PIN       string
}

func fileKind(name, mime string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	mime = strings.ToLower(mime)

	switch {
	case strings.HasPrefix(mime, "image/"), ext == "png", ext == "jpg", ext == "jpeg", ext == "gif", ext == "webp", ext == "svg", ext == "heic", ext == "avif":
		return "image"
	case strings.HasPrefix(mime, "video/"), ext == "mp4", ext == "mov", ext == "mkv", ext == "webm", ext == "avi":
		return "video"
	case strings.HasPrefix(mime, "audio/"), ext == "mp3", ext == "wav", ext == "flac", ext == "ogg", ext == "m4a", ext == "aac":
		return "audio"
	case ext == "zip", ext == "tar", ext == "gz", ext == "tgz", ext == "bz2", ext == "xz", ext == "7z", ext == "rar", ext == "zst":
		return "archive"
	case ext == "go", ext == "rs", ext == "py", ext == "js", ext == "ts", ext == "tsx", ext == "jsx", ext == "java", ext == "c", ext == "cpp", ext == "h", ext == "json", ext == "yaml", ext == "yml", ext == "toml", ext == "xml", ext == "html", ext == "css", ext == "sh", ext == "md":
		return "code"
	default:
		return "file"
	}
}

func buildWebShareFiles(files map[string]model.FileDto) ([]WebShareFile, int64) {
	out := make([]WebShareFile, 0, len(files))
	var total int64
	for id, f := range files {
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(f.FileName), "."))
		out = append(out, WebShareFile{
			ID:       id,
			FileName: f.FileName,
			Size:     f.Size,
			Ext:      ext,
			Kind:     fileKind(f.FileName, f.FileType),
		})
		total += f.Size
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].FileName) < strings.ToLower(out[j].FileName)
	})
	return out, total
}

const webShareEmptyHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="dark">
    <title>LocalGo Share</title>
    <style>
        :root {
            --bg0: #04100a;
            --bg1: #071a10;
            --accent: #4ade80;
            --muted: #9fc2ae;
            --font: 'JetBrains Mono', 'Fira Code', ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
        }
        body {
            margin: 0; min-height: 100vh; display: grid; place-items: center;
            font-family: var(--font); color: #eafdf3;
            background: radial-gradient(900px 500px at 20% 0%, rgba(74,222,128,.15), transparent 55%), var(--bg0);
            padding: 1.25rem;
        }
        .card {
            max-width: 420px; width: 100%; text-align: center;
            background: rgba(12,28,20,.75); border: 1px solid rgba(134,239,172,.12);
            border-radius: 18px; padding: 2rem 1.5rem; box-shadow: 0 25px 50px -12px rgba(0,0,0,.6);
        }
        h1 { margin: 0 0 .5rem; font-size: 1.2rem; }
        p { margin: 0; color: var(--muted); font-size: .95rem; line-height: 1.5; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Nothing shared right now</h1>
        <p>This LocalGo share session has no files available. Ask the sender to start sharing again.</p>
    </div>
</body>
</html>`

// WebShareHandler serves the root web landing page for browser access.
func (h *DownloadHandler) WebShareHandler(w http.ResponseWriter, r *http.Request) {
	session := h.sendService.GetSession()
	if session == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(webShareEmptyHTML))
		return
	}

	pinLocked := false
	pinError := false
	pin := r.URL.Query().Get("pin")
	if h.config.PIN != "" {
		if subtle.ConstantTimeCompare([]byte(pin), []byte(h.config.PIN)) != 1 {
			pinLocked = true
			pinError = pin != ""
		}
	}

	funcMap := template.FuncMap{
		"formatBytes": cli.FormatBytes,
	}

	tmpl, err := template.New("webshare").Funcs(funcMap).Parse(webShareHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	alias := h.config.Alias
	if h.config.Private {
		alias = "Anonymous"
	}

	files, totalSize := buildWebShareFiles(session.Files)
	data := WebShareData{
		Alias:     alias,
		SessionID: session.SessionID,
		Files:     files,
		FileCount: len(files),
		TotalSize: totalSize,
		PinLocked: pinLocked,
		PinError:  pinError,
		PIN:       pin,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = tmpl.Execute(w, data)
}

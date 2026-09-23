package command

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	running  = make(map[string]bool)
	wakes    = make(map[string]chan struct{})
	workerMu sync.Mutex
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

var videoExts = map[string]bool{
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".webm": true,
}

func (c *Commander) ensureWorker(d Display) {
	workerMu.Lock()
	defer workerMu.Unlock()
	if running[d.Name] {
		return
	}
	running[d.Name] = true
	ch := make(chan struct{}, 1)
	wakes[d.Name] = ch
	go c.runWorker(d.Name, ch)
}

// StartWorkers sobe os workers dos displays com lib atribuída (theme != 0).
// Chamado no boot da aplicação — sem esperar a UI.
func (c *Commander) StartWorkers() {
	displays, err := c.listDisplays()
	if err != nil {
		return
	}
	for _, d := range displays {
		if d.Theme != 0 {
			c.ensureWorker(d)
		}
	}
}

// signalWorker acorda o worker (timer/theme editado na UI).
func signalWorker(name string) {
	workerMu.Lock()
	ch := wakes[name]
	workerMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// signalAllWorkers acorda todos (ex.: library deletada).
func signalAllWorkers() {
	workerMu.Lock()
	chs := make([]chan struct{}, 0, len(wakes))
	for _, ch := range wakes {
		chs = append(chs, ch)
	}
	workerMu.Unlock()
	for _, ch := range chs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (c *Commander) runWorker(name string, wake <-chan struct{}) {
	defer func() {
		workerMu.Lock()
		delete(running, name)
		delete(wakes, name)
		workerMu.Unlock()
	}()

	// 0 = omarchy default já considerado "aplicado" — evita restore no boot
	lastAppliedTheme := 0
	currentFile := 0
	missing := 0
	first := true

	for {
		d, ok, err := c.getDisplay(name)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if !ok {
			// display saiu do banco → encerra worker
			missing++
			if missing >= 3 {
				return
			}
			time.Sleep(5 * time.Second)
			continue
		}
		missing = 0

		// primeiro tick aplica na hora; depois espera timer OU wake da UI
		if !first {
			timer := d.Timer
			if timer < 1 {
				timer = 60
			}
			timerC := time.After(time.Duration(timer) * time.Second)
			select {
			case <-timerC:
			case <-wake:
			}
		}
		first = false

		// relê do banco a cada tick: reflete mudanças de theme/timer feitas na UI
		d, ok, err = c.getDisplay(name)
		if err != nil || !ok {
			continue
		}

		if d.Theme == 0 {
			if lastAppliedTheme != 0 {
				_ = c.restoreOmarchyDefault(d.Name)
				lastAppliedTheme = 0
				currentFile = 0
			}
			continue
		}

		// troca de library reseta o giro
		if lastAppliedTheme != d.Theme {
			currentFile = 0
			lastAppliedTheme = d.Theme
		}

		status, err := c.loadStatus()
		if err != nil {
			status = probeStatus()
			_ = c.saveStatus(status)
		}

		libs, err := c.mapLibraries()
		if err != nil {
			continue
		}
		libPath := libs[d.Theme]
		if libPath == "" {
			// library deletada — outbox deve setar theme=0; defensivo aqui
			_ = c.restoreOmarchyDefault(d.Name)
			lastAppliedTheme = 0
			currentFile = 0
			continue
		}

		files := listWallpaperFiles(d, libPath)
		if len(files) == 0 {
			continue
		}

		// wrap infinito — não trava após o último arquivo
		file := files[currentFile%len(files)]
		currentFile++

		ext := strings.ToLower(filepath.Ext(file))
		var applyErr error
		if videoExts[ext] {
			applyErr = tryMpvpaper(d.Name, file, true)
		} else {
			applyErr = c.applyImage(d.Name, file, status)
		}
		if applyErr != nil {
			fmt.Printf("[worker %s] %v\n", name, applyErr)
		}
	}
}

// listWallpaperFiles: arquivos suportados, sem thumbs/, ordenados.
// includeVideos=false quando d.Video está desligado.
func listWallpaperFiles(display Display, root string) []string {
	includeVideos := display.Video

	if root == "" {
		return nil
	}
	thumbsDir := filepath.Join(root, "thumbs")
	var files []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if p == thumbsDir {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Dir(p) == thumbsDir {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if imageExts[ext] {
			files = append(files, p)
			return nil
		}
		if includeVideos && videoExts[ext] {
			files = append(files, p)
		}
		return nil
	})

	src := rand.NewSource(display.Seed)
	r := rand.New(src)
	r.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	return files
}

func (c *Commander) applyImage(monitor, abs string, status Status) error {
	// seta a imagem NOVA por baixo do mpvpaper primeiro — ao matar o vídeo,
	// revela a imagem nova (senão pisca a anterior)
	setErr := c.setWallpaper(monitor, abs, status)
	// dá um frame pro comporitor compor a imagem nova antes de derrubar o vídeo
	if setErr == nil {
		time.Sleep(50 * time.Millisecond)
	}
	_ = stopMpvpaperForMonitor(monitor)
	return setErr
}

func (c *Commander) setWallpaper(monitor, abs string, status Status) error {
	if status.Hyprpaper {
		if err := tryHyprpaper(monitor, abs); err == nil {
			return nil
		}
		// hyprpaper falhou → cai pro omarchy
	}

	cmd := exec.Command("omarchy", "theme", "bg", "set", abs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("omarchy theme bg set falhou: %w", err)
	}
	_ = exec.Command("omarchy-shell", "-q", "background", "set", abs).Run()
	return nil
}

func tryHyprpaper(monitor, abs string) error {
	// garante que o daemon está rodando (hyprpaper 0.8.4 só tem wallpaper, não listloaded/preload)
	if err := exec.Command("pgrep", "-x", "hyprpaper").Run(); err != nil {
		fmt.Println("[hyprpaper] daemon não está rodando, iniciando...")
		cmd := exec.Command("hyprpaper")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("falha ao iniciar hyprpaper: %w", err)
		}
		_ = exec.Command("sleep", "0.8").Run()
	}

	// API 0.8.4: hyprctl hyprpaper wallpaper "MON,PATH[,fit]"
	spec := fmt.Sprintf("%s,%s", monitor, abs)
	//fmt.Printf("[hyprpaper] wallpaper %s\n", spec)
	cmdWall := exec.Command("hyprctl", "hyprpaper", "wallpaper", spec)
	cmdWall.Stdout = os.Stdout
	cmdWall.Stderr = os.Stderr
	if err := cmdWall.Run(); err != nil {
		return fmt.Errorf("wallpaper falhou: %w", err)
	}
	return nil
}

func tryMpvpaper(monitor, abs string, isVideo bool) error {
	if err := stopMpvpaperForMonitor(monitor); err != nil {
		fmt.Printf("[mpvpaper] aviso ao parar anterior: %v\n", err)
	}

	// -p = auto-pause quando oculto, -f = fork (daemoniza)
	// -l bottom garante ficar acima do hyprpaper (background) quando ambos rodam
	layer := "bottom"
	if !isVideo {
		layer = "background"
	}
	mpvOpts := "no-audio loop hwdec=auto"
	if isVideo {
		mpvOpts = "no-audio loop hwdec=auto video-unscaled=no scale=ewa_lanczossharp"
	}
	args := []string{"-p", "-f", "-l", layer, "-o", mpvOpts, monitor, abs}
	fmt.Printf("[mpvpaper] %s\n", strings.Join(append([]string{"mpvpaper"}, args...), " "))
	cmd := exec.Command("mpvpaper", args...)
	// NÃO usar CombinedOutput: com -f + loop o filho herda os pipes e o
	// Wait fica preso até o vídeo "acabar" (nunca) — worker não avança de arquivo.
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpvpaper start: %w", err)
	}
	go func() { _ = cmd.Wait() }() // reap zombie sem bloquear o worker

	// espera curta só para o layer subir — não bloqueia o giro do worker
	_ = exec.Command("sleep", "0.5").Run()

	// só confirma no monitor alvo — sem fallback genérico (pegava outro monitor)
	if err := exec.Command("pgrep", "-f", monitorPattern(monitor)).Run(); err != nil {
		return fmt.Errorf("mpvpaper não parece estar rodando em %s após start", monitor)
	}
	fmt.Printf("[mpvpaper] live wallpaper ativo em %s -> %s\n", monitor, filepath.Base(abs))
	return nil
}

// monitor como argumento próprio (entre espaços) — DP-1 não casa com DP-10
func monitorPattern(monitor string) string {
	return "mpvpaper.* " + monitor + " "
}

func stopMpvpaperForMonitor(monitor string) error {
	out, _ := exec.Command("pgrep", "-f", monitorPattern(monitor)).Output()
	pids := strings.Fields(string(out))
	if len(pids) == 0 {
		return nil
	}
	fmt.Printf("[mpvpaper] parando instância anterior para %s (pids: %s)\n", monitor, strings.Join(pids, ","))
	for _, pid := range pids {
		_ = exec.Command("kill", pid).Run()
	}
	// espera até sumir (máx ~300ms) em vez de sleep fixo de 0.3s
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := exec.Command("pgrep", "-f", monitorPattern(monitor)).Run(); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (c *Commander) mapLibraries() (map[int]string, error) {
	libs, err := c.fetchLibraries()
	if err != nil {
		return nil, err
	}
	mapa := make(map[int]string, len(libs))
	for i := range libs {
		mapa[libs[i].ID] = libs[i].Path
	}
	return mapa, nil
}

// restoreOmarchyDefault volta ao wallpaper global do omarchy APENAS para este monitor.
func (c *Commander) restoreOmarchyDefault(monitor string) error {
	themeName := getCurrentThemeName()
	fmt.Printf("[undo %s] tema atual: %s\n", monitor, themeName)

	defaultPath := findDefaultWallpaper(themeName)
	if defaultPath == "" {
		return fmt.Errorf("nenhum wallpaper padrão encontrado para tema %s", themeName)
	}
	fmt.Printf("[undo %s] wallpaper padrão: %s\n", monitor, defaultPath)

	status, err := c.loadStatus()
	if err != nil {
		status = probeStatus()
	}

	// seta o padrão ANTES de matar o vídeo — evita flash do wallpaper anterior
	if status.Hyprpaper {
		_ = tryHyprpaper(monitor, defaultPath)
	}

	// omarchy global (background compartilhado)
	if _, err := exec.LookPath("omarchy"); err == nil {
		fmt.Printf("[undo %s] omarchy theme bg set %s\n", monitor, filepath.Base(defaultPath))
		cmd := exec.Command("omarchy", "theme", "bg", "set", defaultPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("[warn] omarchy bg set falhou: %v, tentando symlink manual\n", err)
		} else {
			_ = exec.Command("omarchy-shell", "-q", "background", "set", defaultPath).Run()
			time.Sleep(50 * time.Millisecond)
			_ = stopMpvpaperForMonitor(monitor)
			return nil
		}
	}

	// fallback symlink manual
	home, _ := os.UserHomeDir()
	link := filepath.Join(home, ".local/state/omarchy/current/background")
	_ = os.MkdirAll(filepath.Dir(link), 0o755)
	_ = os.Remove(link)
	if err := os.Symlink(defaultPath, link); err != nil {
		return fmt.Errorf("falha ao criar symlink %s -> %s: %w", link, defaultPath, err)
	}
	fmt.Printf("[undo %s] symlink %s -> %s\n", monitor, link, defaultPath)
	cmd := exec.Command("omarchy-shell", "-q", "background", "set", defaultPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	time.Sleep(50 * time.Millisecond)
	// só este monitor — não derruba mpvpaper/hyprpaper dos outros
	_ = stopMpvpaperForMonitor(monitor)
	return nil
}

func getCurrentThemeName() string {
	if data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".local/state/omarchy/current/theme.name")); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			return name
		}
	}
	if out, err := exec.Command("omarchy", "theme", "current").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			lower := strings.ToLower(name)
			lower = strings.ReplaceAll(lower, " ", "-")
			return lower
		}
	}
	return "tokyo-night"
}

func findDefaultWallpaper(themeName string) string {
	home := os.Getenv("HOME")
	candidates := []string{
		filepath.Join(home, ".local/state/omarchy/current/theme/backgrounds"),
		filepath.Join(home, ".config/omarchy/backgrounds", themeName),
		filepath.Join("/usr/share/omarchy/themes", themeName, "backgrounds"),
	}
	if out, err := exec.Command("omarchy", "theme", "dir", themeName).Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			candidates = append([]string{filepath.Join(dir, "backgrounds")}, candidates...)
		}
	}

	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			low := strings.ToLower(e.Name())
			if imageExts[filepath.Ext(low)] || strings.HasSuffix(low, ".bmp") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
		if len(files) > 0 {
			sort.Strings(files)
			return files[0]
		}
	}
	return ""
}

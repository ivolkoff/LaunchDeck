package i18n

// catalog maps a dot-namespaced key to its English and Russian text. The en
// value of every entry MUST be byte-for-byte the string in the code today (the
// English invariant that keeps the existing suite green). Format entries use
// fmt verbs consumed by Tf.
var catalog = map[string]struct{ en, ru string }{
	// --- status line (internal/app/reduce.go) ---
	"status.infer_domain": {"cannot infer domain (path is under neither LaunchAgents nor LaunchDaemons)", "не удалось определить домен (путь не в LaunchAgents и не в LaunchDaemons)"},
	"status.load":         {"load…", "загрузка…"},
	"status.timeout":      {"%s timed out", "%s: таймаут"},
	"status.ok":           {"%s ok", "%s: ок"},
	"status.load_perm":    {"load failed (permission denied) — system daemons must be loaded with elevated privileges", "загрузка не удалась (отказано в доступе) — системные демоны загружаются с повышенными привилегиями"},
	"status.needs_sudo":   {"%s needs sudo — Retry with sudo", "%s: нужен sudo — повторить с sudo"},
	"status.failed":       {"%s failed: %s", "%s: ошибка: %s"},
	"status.busy":         {"action already running", "действие уже выполняется"},
	"status.enum_root":    {"system services need root to enumerate (run launchdeck with sudo to see them)", "перечисление системных сервисов требует root (запустите launchdeck под sudo)"},
	"status.parse_fail":   {"failed to parse services", "не удалось разобрать список сервисов"},

	// --- detail error (internal/app/reduce.go) ---
	"detail.err_sudo": {"requires sudo to inspect — run launchdeck with sudo to view system services", "нужен sudo для просмотра — запустите launchdeck под sudo, чтобы видеть системные сервисы"},

	// --- localized action verbs (used inside status/prompt formats) ---
	"action.verb.start":   {"start", "запуск"},
	"action.verb.restart": {"restart", "перезапуск"},
	"action.verb.stop":    {"stop", "остановка"},
	"action.verb.enable":  {"enable", "включение"},
	"action.verb.disable": {"disable", "отключение"},
	"action.verb.unload":  {"unload", "выгрузка"},
	"action.verb.load":    {"load", "загрузка"},
	"action.verb.unknown": {"unknown", "неизвестно"},

	// --- prompts (internal/app/derive.go) ---
	"prompt.sudo":    {"Retry with sudo? (y/n)", "Повторить с sudo? (y/n)"},
	"prompt.confirm": {"%s %s? (y/n)", "%s %s? (y/n)"},
	"prompt.filter":  {"filter: ", "фильтр: "},
	"prompt.load":    {"load plist: ", "загрузить plist: "},
	"prompt.action":  {"action: %s (s/r/k/e/d/u, Enter, Esc)", "действие: %s (s/r/k/e/d/u, Enter, Esc)"},

	// --- list placeholders (internal/app/derive.go) ---
	"list.loading": {"Loading services…", "Загрузка сервисов…"},
	"list.empty":   {"No matching services", "Нет подходящих сервисов"},
	"list.gone":    {" (gone)", " (удалён)"},

	// --- run / enable state words (internal/app/derive.go) ---
	"runstate.running": {"running", "работает"},
	"runstate.stopped": {"stopped", "остановлен"},
	"enable.enabled":   {"enabled", "включён"},
	"enable.disabled":  {"disabled", "отключён"},

	// --- log note (internal/app/derive.go) ---
	"log.none": {"no log configured", "лог не настроен"},

	// --- action-button display labels (internal/ui/bubbletea/statusbar.go) ---
	"btn.Start":   {"Start", "Запуск"},
	"btn.Restart": {"Restart", "Перезапуск"},
	"btn.Stop":    {"Stop", "Стоп"},
	"btn.Enable":  {"Enable", "Вкл"},
	"btn.Disable": {"Disable", "Выкл"},
	"btn.Unload":  {"Unload", "Выгрузка"},

	// --- detail panel (internal/ui/bubbletea/detail.go) ---
	"detail.select":  {"Select a service", "Выберите сервис"},
	"detail.loading": {"Loading detail…", "Загрузка деталей…"},
	"detail.gone":    {"(gone) — service no longer present", "(удалён) — сервиса больше нет"},

	// --- metadata labels (colon + alignment added in code) ---
	"meta.label":   {"Label", "Метка"},
	"meta.domain":  {"Domain", "Домен"},
	"meta.pid":     {"PID", "PID"},
	"meta.exit":    {"Last exit", "Выход"},
	"meta.run":     {"Run", "Статус"},
	"meta.enable":  {"Enable", "Включён"},
	"meta.program": {"Program", "Программа"},
	"meta.plist":   {"Plist", "Plist"},

	// --- detail tab display names (zone id stays English) ---
	"tab.Metadata": {"Metadata", "Метаданные"},
	"tab.Logs":     {"Logs", "Логи"},
	"tab.Raw":      {"Raw", "Raw"},

	// --- header + too-small (internal/ui/bubbletea/view.go) ---
	"view.too_small": {"terminal too small (need ≥60×20)", "терминал слишком мал (нужно ≥60×20)"},
	"header.title":   {" LaunchDeck — launchctl services · ? help", " LaunchDeck — сервисы launchctl · ? справка"},

	// --- help overlay (internal/ui/bubbletea/view.go) ---
	"help.title":           {"LaunchDeck — help", "LaunchDeck — справка"},
	"help.nav":             {"Navigation", "Навигация"},
	"help.nav.move":        {"  ↑/k ↓/j      move selection (sidebar) · scroll (detail, by focus)", "  ↑/k ↓/j      выбор (меню) · прокрутка (детали, по фокусу)"},
	"help.nav.homeend":     {"  Home/End     first / last row", "  Home/End     первая / последняя строка"},
	"help.nav.page":        {"  PgUp/PgDn    page up / down", "  PgUp/PgDn    страница вверх / вниз"},
	"help.nav.tab":         {"  Tab          switch focus: sidebar ↔ detail", "  Tab          фокус: меню ↔ детали"},
	"help.nav.tabs":        {"  1/2/3  ←/→   detail tabs: Metadata / Logs / Raw", "  1/2/3  ←/→   вкладки: Метаданные / Логи / Raw"},
	"help.nav.scroll":      {"  Ctrl-U/D     scroll detail ±10   ·   mouse wheel ±3", "  Ctrl-U/D     прокрутка деталей ±10  ·  колесо ±3"},
	"help.actions":         {"Actions", "Действия"},
	"help.actions.suffix":  {" (on the selected service)", " (над выбранным сервисом)"},
	"help.actions.picker1": {"  a            action picker → s start · r restart · k stop", "  a            меню действий → s запуск · r перезапуск · k стоп"},
	"help.actions.picker2": {"               e enable · d disable · u unload", "               e включить · d отключить · u выгрузить"},
	"help.actions.confirm": {"  y/Enter n/Esc  confirm / cancel a prompt", "  y/Enter n/Esc  подтвердить / отменить"},
	"help.actions.load":    {"  L            load a plist (bootstrap)", "  L            загрузить plist (bootstrap)"},
	"help.view":            {"View", "Вид"},
	"help.view.filter":     {"  / f          filter (regex, live)   d   user ↔ user+system", "  / f          фильтр (regex, живой)  d   user ↔ user+system"},
	"help.view.sort":       {"  s / S        sort key / direction  r   refresh now", "  s / S        сортировка / порядок  r   обновить"},
	"help.view.mouse":      {"  m            capture mouse (click · wheel · divider) — off = drag selects text", "  m            захват мыши (клик · колесо · разделитель) — выкл = drag выделяет"},
	"help.view.help":       {"  ?            this help             q / Ctrl-C  quit (saves)", "  ?            эта справка           q / Ctrl-C  выход (сохр.)"},
	"help.mouse":           {"Mouse", "Мышь"},
	"help.mouse.desc":      {"  click rows/tabs/buttons · wheel scroll · drag the divider to resize", "  клики строк/вкладок/кнопок · колесо · тяните разделитель"},
	"help.footer":          {"press ? or Esc to close", "нажмите ? или Esc чтобы закрыть"},
}

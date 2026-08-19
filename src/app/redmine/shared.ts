// Хелперы, общие для дашборда (page.tsx) и страницы отдельного проекта
// ([project]/page.tsx) — вынесены сюда, чтобы не дублировать и не рисковать
// рассинхроном (см. историю: canHaveCycle и eventVerb уже расходились между
// двумя файлами до вынесения).
import { RedmineProjectControlEvent, RedmineProjectDashboardItem, RedmineProjectGroup, RedmineProjectType } from "@/lib/api"

export function groupKeyForItem(item: RedmineProjectDashboardItem) {
  const name = item.group_name.toLowerCase()
  if (name.includes("актив")) return "active"
  if (name.includes("пауз")) return "pause"
  if (name.includes("заверш")) return "done"
  return "unknown"
}

export function groupForColumn(groups: RedmineProjectGroup[], key: string) {
  if (key === "active") return groups.find((group) => group.name.toLowerCase().includes("актив"))
  if (key === "pause") return groups.find((group) => group.name.toLowerCase().includes("пауз"))
  if (key === "done") return groups.find((group) => group.name.toLowerCase().includes("заверш"))
  return null
}

export const PROJECT_TYPES: Array<{ key: RedmineProjectType; label: string }> = [
  { key: "", label: "Не задан" },
  { key: "seo", label: "SEO" },
  { key: "ads", label: "Реклама" },
  { key: "dev", label: "Разработка" },
  { key: "legal", label: "Юридическая помощь" },
  { key: "support", label: "Техподдержка" },
]

export function projectTypeLabel(type: RedmineProjectType) {
  return PROJECT_TYPES.find((item) => item.key === type)?.label || "Не задан"
}

// eventSentActionLabel is the button text for marking a control event as
// sent — a call to action, not a status (that's eventStatusLabel in
// [project]/page.tsx).
export function eventSentActionLabel(event: RedmineProjectControlEvent) {
  if (event.event_type === "control_cut") return "Отметить КС отправленным"
  // dev cycles store their roadmap milestone as event_type "report_date"
  // too (only the title differs — see insertCycleEvents on the backend),
  // so "Отметить ОД отправленной" would be wrong for them.
  if (event.event_type === "report_date") {
    return event.service_type === "dev" ? "Отметить этап закрытым" : "Отметить ОД отправленной"
  }
  return "Отметить этап закрытым"
}

export function nextMonthDate() {
  const date = new Date()
  date.setMonth(date.getMonth() + 1)
  return date.toISOString().slice(0, 10)
}

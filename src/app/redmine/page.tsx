"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import {
  AlertTriangle,
  ArrowLeft,
  CalendarDays,
  CheckCircle2,
  ExternalLink,
  FileText,
  Flame,
  Loader2,
  PauseCircle,
  Phone,
  RefreshCw,
  Search,
  Settings,
  Trash2,
  UserRound,
  X,
} from "lucide-react"

import {
  redmineAPI,
  RedmineDeadlineState,
  RedmineProjectControlEvent,
  RedmineProjectDashboardItem,
  RedmineProjectDashboardResponse,
  RedmineProjectGroup,
  RedmineProjectType,
} from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { eventSentActionLabel, groupForColumn, groupKeyForItem, nextMonthDate, PROJECT_TYPES, projectTypeLabel } from "./shared"

const STATUS_COLUMNS = [
  { key: "active", title: "Активные", tone: "bg-success", icon: CheckCircle2 },
  { key: "pause", title: "Пауза", tone: "bg-warning", icon: PauseCircle },
  { key: "done", title: "Завершенные", tone: "bg-neutral-500", icon: CheckCircle2 },
  { key: "unknown", title: "Не разобраны", tone: "bg-neutral-400", icon: UserRound },
]

const DEADLINE_STATES: Record<RedmineDeadlineState, { label: string; className: string; icon: typeof CheckCircle2 }> = {
  ok: { label: "Ок", className: "border-success/30 bg-success/10 text-success", icon: CheckCircle2 },
  due_soon: { label: "Скоро срок", className: "border-warning/30 bg-warning/10 text-warning", icon: AlertTriangle },
  burning: { label: "Горит", className: "border-destructive/30 bg-destructive/10 text-destructive", icon: Flame },
  urgent: { label: "Срочное", className: "border-destructive/50 bg-destructive/20 text-destructive", icon: Flame },
}

function formatDate(value: string) {
  if (!value) return ""
  const date = new Date(`${value}T00:00:00`)
  return date.toLocaleDateString("ru-RU", { day: "numeric", month: "short" })
}

function daysUntil(value: string) {
  if (!value) return null
  const due = new Date(`${value}T00:00:00`)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((due.getTime() - today.getTime()) / 86400000)
}

export default function RedmineDashboardPage() {
  const router = useRouter()
  const [items, setItems] = useState<RedmineProjectDashboardItem[]>([])
  const [groups, setGroups] = useState<RedmineProjectGroup[]>([])
  const [managers, setManagers] = useState<{ id: string; name: string }[]>([])
  const [syncedAt, setSyncedAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [savingProjectId, setSavingProjectId] = useState("")
  const [error, setError] = useState("")
  const [searchQuery, setSearchQuery] = useState("")
  const [managerFilter, setManagerFilter] = useState("")
  const [statusFilter, setStatusFilter] = useState("")
  const [typeFilter, setTypeFilter] = useState<RedmineProjectType>("")
  const [deadlineFilter, setDeadlineFilter] = useState("")
  const [withoutTypeOnly, setWithoutTypeOnly] = useState(false)
  const [missingCycleOnly, setMissingCycleOnly] = useState(false)
  const [viewMode, setViewMode] = useState<"status" | "manager">("status")
  const [cardMode, setCardMode] = useState<"lead" | "manager" | "docs" | "compact">("lead")
  const [dragProjectId, setDragProjectId] = useState("")
  const [cycleDates, setCycleDates] = useState<Record<string, string>>({})
  const [deleteEventTarget, setDeleteEventTarget] = useState<{ item: RedmineProjectDashboardItem; event: RedmineProjectControlEvent } | null>(null)

  const applyDashboard = useCallback((response: RedmineProjectDashboardResponse) => {
    setItems(response.data || [])
    setGroups(response.groups || [])
    setManagers([...(response.managers || [])].sort((left, right) => left.name.localeCompare(right.name, "ru")))
    setSyncedAt(response.synced_at)
  }, [])

  const loadDashboard = useCallback(async (showLoader = true) => {
    if (showLoader) setLoading(true)
    setError("")
    try {
      const response = await redmineAPI.getDashboard()
      applyDashboard(response)
    } catch (err: unknown) {
      console.error("Failed to load Redmine dashboard:", err)
      setError(err instanceof Error ? err.message : "Не удалось загрузить dashboard Redmine")
    } finally {
      if (showLoader) setLoading(false)
    }
  }, [applyDashboard])

  useEffect(() => {
    loadDashboard()
  }, [loadDashboard])

  const filteredItems = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase()
    return items.filter((item) => {
      const matchesSearch = !needle ||
        item.name.toLowerCase().includes(needle) ||
        item.identifier.toLowerCase().includes(needle) ||
        item.description.toLowerCase().includes(needle)
      // Keyed by id-or-name (matching the `managers` option keying below) so
      // two managers sharing a display name filter independently instead of
      // both matching whichever one the filter's raw name string picks.
      const matchesManager = !managerFilter || (item.effective_manager_id || item.effective_manager_name) === managerFilter
      const matchesStatus = !statusFilter || groupKeyForItem(item) === statusFilter
      const matchesType = !typeFilter || item.project_type === typeFilter
      const matchesDeadline = !deadlineFilter || item.deadline_state === deadlineFilter
      const matchesWithoutType = !withoutTypeOnly || !item.project_type
      const matchesMissingCycle = !missingCycleOnly || (!item.next_control_event && Boolean(item.project_type))
      return matchesSearch && matchesManager && matchesStatus && matchesType && matchesDeadline && matchesWithoutType && matchesMissingCycle
    })
  }, [deadlineFilter, items, managerFilter, missingCycleOnly, searchQuery, statusFilter, typeFilter, withoutTypeOnly])

  const itemsByColumn = useMemo(() => {
    const result: Record<string, RedmineProjectDashboardItem[]> = {
      active: [],
      pause: [],
      done: [],
      unknown: [],
    }
    for (const item of filteredItems) {
      result[groupKeyForItem(item)].push(item)
    }
    return result
  }, [filteredItems])

  const itemsByManager = useMemo(() => {
    const result: Record<string, RedmineProjectDashboardItem[]> = {}
    for (const item of filteredItems) {
      const manager = item.effective_manager_name || "Без проектника"
      if (!result[manager]) result[manager] = []
      result[manager].push(item)
    }
    return Object.entries(result)
      .sort(([left], [right]) => {
        if (left === "Без проектника") return 1
        if (right === "Без проектника") return -1
        return left.localeCompare(right, "ru")
      })
      .map(([manager, list]) => ({ manager, list }))
  }, [filteredItems])

  const summary = useMemo(() => {
    const result = {
      active: 0,
      urgent: 0,
      burning: 0,
      dueSoon: 0,
      withoutType: 0,
      withoutCycle: 0,
    }
    for (const item of items) {
      if (groupKeyForItem(item) === "active") result.active += 1
      if (item.deadline_state === "urgent") result.urgent += 1
      if (item.deadline_state === "burning") result.burning += 1
      if (item.deadline_state === "due_soon") result.dueSoon += 1
      if (!item.project_type) result.withoutType += 1
      if (!item.next_control_event && item.project_type) {
        result.withoutCycle += 1
      }
    }
    return result
  }, [items])

  const activeFilterChips = useMemo(() => {
    const chips: { key: string; label: string; onRemove: () => void }[] = []
    if (searchQuery) chips.push({ key: "search", label: `Поиск: «${searchQuery}»`, onRemove: () => setSearchQuery("") })
    if (managerFilter) {
      const managerLabel = managers.find((manager) => (manager.id || manager.name) === managerFilter)?.name || managerFilter
      chips.push({ key: "manager", label: `Проектник: ${managerLabel}`, onRemove: () => setManagerFilter("") })
    }
    if (typeFilter) chips.push({ key: "type", label: `Тип: ${projectTypeLabel(typeFilter)}`, onRemove: () => setTypeFilter("") })
    if (statusFilter) {
      const columnTitle = STATUS_COLUMNS.find((column) => column.key === statusFilter)?.title || statusFilter
      chips.push({ key: "status", label: `Статус: ${columnTitle}`, onRemove: () => setStatusFilter("") })
    }
    if (deadlineFilter) {
      const stateLabel = DEADLINE_STATES[deadlineFilter as RedmineDeadlineState]?.label || deadlineFilter
      chips.push({ key: "deadline", label: `Срок: ${stateLabel}`, onRemove: () => setDeadlineFilter("") })
    }
    if (withoutTypeOnly) chips.push({ key: "withoutType", label: "Без типа", onRemove: () => setWithoutTypeOnly(false) })
    if (missingCycleOnly) chips.push({ key: "missingCycle", label: "Без цикла", onRemove: () => setMissingCycleOnly(false) })
    return chips
  }, [searchQuery, managerFilter, managers, typeFilter, statusFilter, deadlineFilter, withoutTypeOnly, missingCycleOnly])

  const countsByColumn = useMemo(() => {
    const result: Record<string, number> = {
      active: 0,
      pause: 0,
      done: 0,
      unknown: 0,
    }
    for (const item of items) {
      result[groupKeyForItem(item)] += 1
    }
    return result
  }, [items])

  const handleSync = async () => {
    setSyncing(true)
    setError("")
    try {
      const response = await redmineAPI.syncDashboard()
      applyDashboard(response)
    } catch (err: unknown) {
      console.error("Failed to sync Redmine dashboard:", err)
      setError(err instanceof Error ? err.message : "Не удалось синхронизировать Redmine")
    } finally {
      setSyncing(false)
    }
  }

  const updateLocalItem = (projectId: string, patch: Partial<RedmineProjectDashboardItem>) => {
    setItems((current) => current.map((item) => (
      item.project_id === projectId ? { ...item, ...patch } : item
    )))
  }

  const handleStatusChange = async (item: RedmineProjectDashboardItem, nextKey: string) => {
    const group = groupForColumn(groups, nextKey)
    const groupId = group?.id || ""
    setError("")
    try {
      await redmineAPI.assignProjectGroup(item.project_id, groupId)
      updateLocalItem(item.project_id, {
        group_id: groupId,
        group_name: group?.name || "",
        group_color: group?.color || "",
        group_assigned_manually: true,
      })
    } catch (err: unknown) {
      console.error("Failed to update project status:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить статус проекта")
    }
  }

  const handleDropToColumn = async (nextKey: string) => {
    const item = items.find((project) => project.project_id === dragProjectId)
    setDragProjectId("")
    if (!item || groupKeyForItem(item) === nextKey) return
    await handleStatusChange(item, nextKey)
  }

  const handleManagerChange = async (item: RedmineProjectDashboardItem, managerId: string, managerName: string) => {
    setError("")
    try {
      await redmineAPI.assignProjectManager(item.project_id, { manager_id: managerId, manager_name: managerName })
      updateLocalItem(item.project_id, {
        manual_manager_name: managerName,
        manual_manager_id: managerId,
        effective_manager_name: managerName || item.inferred_manager_name,
        effective_manager_id: managerName ? managerId : item.inferred_manager_id,
      })
    } catch (err: unknown) {
      console.error("Failed to update project manager:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить проектника")
    }
  }

  const handleProjectTypeChange = async (item: RedmineProjectDashboardItem, projectType: RedmineProjectType) => {
    setSavingProjectId(item.project_id)
    setError("")
    try {
      await redmineAPI.updateProjectOperations(item.project_id, { project_type: projectType })
      updateLocalItem(item.project_id, { project_type: projectType })
    } catch (err: unknown) {
      console.error("Failed to update project type:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить тип проекта")
    } finally {
      setSavingProjectId("")
    }
  }

  const handleUrgentToggle = async (item: RedmineProjectDashboardItem) => {
    const nextUrgent = !item.urgent
    let reason = ""
    if (nextUrgent) {
      const promptResult = window.prompt("Причина срочности", item.urgent_reason || "")
      if (promptResult === null) return // user clicked Cancel — don't mark urgent
      reason = promptResult
    }
    setSavingProjectId(item.project_id)
    setError("")
    try {
      await redmineAPI.updateProjectOperations(item.project_id, { urgent: nextUrgent, urgent_reason: reason })
      updateLocalItem(item.project_id, {
        urgent: nextUrgent,
        urgent_reason: reason,
        deadline_state: nextUrgent ? "urgent" : item.next_control_event ? item.deadline_state : "ok",
      })
    } catch (err: unknown) {
      console.error("Failed to update project urgency:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить срочность")
    } finally {
      setSavingProjectId("")
    }
  }

  const handleGenerateCycle = async (item: RedmineProjectDashboardItem) => {
    const reportDate = cycleDates[item.project_id] || nextMonthDate()
    const projectType = item.project_type
    if (!projectType) {
      setError("Сначала задайте тип проекта")
      return
    }
    setSavingProjectId(item.project_id)
    setError("")
    try {
      await redmineAPI.generateProjectCycle(item.project_id, { project_type: projectType, report_date: reportDate })
      await loadDashboard(false)
    } catch (err: unknown) {
      console.error("Failed to generate project cycle:", err)
      setError(err instanceof Error ? err.message : "Не удалось создать цикл дат")
    } finally {
      setSavingProjectId("")
    }
  }

  const handleEventSent = async (item: RedmineProjectDashboardItem, event: RedmineProjectControlEvent) => {
    setSavingProjectId(item.project_id)
    setError("")
    try {
      await redmineAPI.markControlEventSent(item.project_id, event.id, { sent_by: item.effective_manager_name })
      await loadDashboard(false)
    } catch (err: unknown) {
      console.error("Failed to mark control event sent:", err)
      setError(err instanceof Error ? err.message : "Не удалось отметить событие отправленным")
    } finally {
      setSavingProjectId("")
    }
  }

  const confirmEventDelete = async () => {
    if (!deleteEventTarget) return
    const { item, event } = deleteEventTarget
    setSavingProjectId(item.project_id)
    setError("")
    try {
      await redmineAPI.deleteControlEvent(item.project_id, event.id)
      await loadDashboard(false)
      setDeleteEventTarget(null)
    } catch (err: unknown) {
      console.error("Failed to delete control event:", err)
      setError(err instanceof Error ? err.message : "Не удалось удалить контрольную дату")
    } finally {
      setSavingProjectId("")
    }
  }

  const renderProject = (item: RedmineProjectDashboardItem) => {
    const currentKey = groupKeyForItem(item)
    const managerName = item.manual_manager_name || item.effective_manager_name
    const managerId = item.manual_manager_id || item.effective_manager_id
    // Option values are the manager ID when known, falling back to the name
    // for legacy id-less records — matches the keying in `managers` below.
    const managerValue = managerId || managerName
    const state = DEADLINE_STATES[item.deadline_state] || DEADLINE_STATES.ok
    const StateIcon = state.icon
    const nextEvent = item.next_control_event
    const days = nextEvent ? daysUntil(nextEvent.due_date) : null
    const isSaving = savingProjectId === item.project_id
    // The backend (insertCycleEvents) generates a cycle for any assigned
    // project type, not just seo/ads/support — matches [project]/page.tsx.
    const canHaveCycle = Boolean(item.project_type)

    return (
      <article
        key={item.project_id}
        draggable
        role="button"
        tabIndex={0}
        onClick={() => router.push(`/redmine/${item.project_id}`)}
        onKeyDown={(event) => {
          // Ignore keydowns bubbling up from nested interactive controls
          // (manager select, urgent toggle, etc.) — only the card itself
          // should navigate on Enter/Space.
          if (event.target !== event.currentTarget) return
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault()
            router.push(`/redmine/${item.project_id}`)
          }
        }}
        onDragStart={(event) => {
          if ((event.target as HTMLElement).closest("[data-interactive='true']")) {
            event.preventDefault()
            return
          }
          setDragProjectId(item.project_id)
          event.dataTransfer.effectAllowed = "move"
        }}
        onDragEnd={() => setDragProjectId("")}
        className={`cursor-pointer rounded-md border bg-background p-4 shadow-sm transition-colors hover:border-primary/50 hover:bg-muted/30 ${
          item.deadline_state === "burning" || item.deadline_state === "urgent" ? "border-destructive/40" : ""
        }`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">{item.name}</div>
            <div className="mt-1 text-xs text-muted-foreground">{item.identifier}</div>
          </div>
          {item.url && (
            <a
              href={item.url}
              target="_blank"
              rel="noreferrer"
              aria-label="Открыть проект в Redmine"
              data-interactive="true"
              onClick={(event) => event.stopPropagation()}
            >
              <ExternalLink className="h-4 w-4 text-muted-foreground" />
            </a>
          )}
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          <span className="rounded border px-2 py-1 text-xs">{projectTypeLabel(item.project_type)}</span>
          <span className={`inline-flex items-center gap-1 rounded border px-2 py-1 text-xs ${state.className}`}>
            <StateIcon className="h-3 w-3" />
            {state.label}
          </span>
        </div>

        {cardMode !== "compact" && (
          <div className="mt-3 text-sm text-muted-foreground">
            Проектник: <span className="text-foreground">{managerName || "не назначен"}</span>
          </div>
        )}

        {item.urgent && item.urgent_reason && cardMode !== "compact" && (
          <Alert className="mt-3" icon={<Flame className="mt-0.5 h-4 w-4 shrink-0" />}>
            Срочно: {item.urgent_reason}
          </Alert>
        )}

        {cardMode !== "docs" && (
          <div className="mt-4 rounded-md border bg-muted/20 p-3">
            {nextEvent ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 text-sm font-medium">
                      <CalendarDays className="h-4 w-4 text-muted-foreground" />
                      {nextEvent.title}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {formatDate(nextEvent.due_date)}
                      {days !== null && (
                        <span> · {days < 0 ? `просрочено ${Math.abs(days)} дн.` : days === 0 ? "сегодня" : `через ${days} дн.`}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant={item.deadline_state === "burning" || item.deadline_state === "urgent" ? "default" : "outline"}
                      disabled={isSaving}
                      data-interactive="true"
                      onClick={(event) => {
                        event.stopPropagation()
                        handleEventSent(item, nextEvent)
                      }}
                    >
                      {isSaving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                      {eventSentActionLabel(nextEvent)}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={isSaving}
                      aria-label="Удалить контрольную дату"
                      data-interactive="true"
                      onClick={(event) => {
                        event.stopPropagation()
                        setDeleteEventTarget({ item, event: nextEvent })
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </div>
            ) : canHaveCycle ? (
              <div className="space-y-2" data-interactive="true" onClick={(event) => event.stopPropagation()}>
                <div className="text-sm font-medium">Цикл не настроен</div>
                <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
                  <Input
                    type="date"
                    value={cycleDates[item.project_id] || nextMonthDate()}
                    onChange={(event) => setCycleDates((current) => ({ ...current, [item.project_id]: event.target.value }))}
                  />
                  <Button size="sm" disabled={isSaving} onClick={() => handleGenerateCycle(item)}>
                    {isSaving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                    Создать цикл
                  </Button>
                </div>
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">Контрольные даты не настроены</div>
            )}
          </div>
        )}

        {cardMode === "docs" && (
          <div className="mt-4 rounded-md border bg-muted/20 p-3 text-sm">
            <div className="flex items-center gap-2 font-medium">
              <FileText className="h-4 w-4 text-muted-foreground" />
              Документы
            </div>
            <div className="mt-2 text-muted-foreground">
              Откройте карточку проекта, чтобы увидеть счета, акты и файлы Redmine.
            </div>
          </div>
        )}

        {cardMode === "manager" && item.inferred_manager_name && item.manual_manager_name && (
          <div className="mt-3 text-xs text-muted-foreground">
            По задачам: {item.inferred_manager_name}
          </div>
        )}

        <details className="mt-4" data-interactive="true" onClick={(event) => event.stopPropagation()}>
          <summary className="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-muted-foreground">
            <Settings className="h-3.5 w-3.5" />
            Настройки
          </summary>
          <div className="mt-3 space-y-3">
            <label className="block space-y-1">
              <span className="text-xs font-medium text-muted-foreground">Тип проекта</span>
              <Select
                value={item.project_type}
                onChange={(event) => handleProjectTypeChange(item, event.target.value as RedmineProjectType)}
                disabled={isSaving}
              >
                {PROJECT_TYPES.map((type) => (
                  <option key={type.key || "none"} value={type.key}>{type.label}</option>
                ))}
              </Select>
            </label>

            <label className="block space-y-1">
              <span className="text-xs font-medium text-muted-foreground">Проектник</span>
              <Select
                value={managerValue}
                onChange={(event) => {
                  const value = event.target.value
                  const selected = managers.find((manager) => (manager.id || manager.name) === value)
                  handleManagerChange(item, selected?.id || "", selected?.name || value)
                }}
              >
                <option value="">Не назначен</option>
                {managers.map((manager) => (
                  <option key={manager.id || manager.name} value={manager.id || manager.name}>{manager.name}</option>
                ))}
              </Select>
            </label>

            <label className="block space-y-1">
              <span className="text-xs font-medium text-muted-foreground">Статус</span>
              <Select
                value={currentKey}
                onChange={(event) => handleStatusChange(item, event.target.value)}
              >
                <option value="active">Активный</option>
                <option value="pause">Пауза</option>
                <option value="done">Завершенный</option>
                <option value="unknown">Не разобран</option>
              </Select>
            </label>

            <Button type="button" variant="outline" size="sm" disabled={isSaving} onClick={() => handleUrgentToggle(item)}>
              {item.urgent ? "Снять срочное" : "Пометить срочным"}
            </Button>
          </div>
        </details>
      </article>
    )
  }

  return (
    <div className="container mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-4">
          <Link href="/">
            <Button variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Главная
            </Button>
          </Link>
          <Link href="/zvonari">
            <Button variant="outline">
              <Phone className="mr-2 h-4 w-4" />
              Звонари
            </Button>
          </Link>
          <div>
            <h1 className="text-3xl font-bold">Проекты Redmine</h1>
            <p className="text-sm text-muted-foreground">
              {syncedAt ? `Данные обновлены: ${new Date(syncedAt).toLocaleString("ru-RU")}` : "Первичная синхронизация еще не выполнена"}
            </p>
          </div>
        </div>
        <Button onClick={handleSync} disabled={syncing || loading}>
          {syncing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
          Синхронизировать Redmine
        </Button>
      </div>

      <p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        Быстрые фильтры — нажмите ещё раз, чтобы снять
      </p>
      <div className="mb-6 grid gap-3 md:grid-cols-3 xl:grid-cols-6">
        <button
          type="button"
          onClick={() => setStatusFilter((current) => (current === "active" ? "" : "active"))}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${statusFilter === "active" ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Активные</div>
          <div className="mt-1 text-2xl font-semibold">{summary.active}</div>
        </button>
        <button
          type="button"
          onClick={() => setDeadlineFilter((current) => (current === "urgent" ? "" : "urgent"))}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${deadlineFilter === "urgent" ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Срочные</div>
          <div className="mt-1 text-2xl font-semibold">{summary.urgent}</div>
        </button>
        <button
          type="button"
          onClick={() => setDeadlineFilter((current) => (current === "burning" ? "" : "burning"))}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${deadlineFilter === "burning" ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Горят</div>
          <div className="mt-1 text-2xl font-semibold">{summary.burning}</div>
        </button>
        <button
          type="button"
          onClick={() => setDeadlineFilter((current) => (current === "due_soon" ? "" : "due_soon"))}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${deadlineFilter === "due_soon" ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Скоро срок</div>
          <div className="mt-1 text-2xl font-semibold">{summary.dueSoon}</div>
        </button>
        <button
          type="button"
          onClick={() => {
            setWithoutTypeOnly((current) => !current)
            setTypeFilter("")
          }}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${withoutTypeOnly ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Без типа</div>
          <div className="mt-1 text-2xl font-semibold">{summary.withoutType}</div>
        </button>
        <button
          type="button"
          onClick={() => setMissingCycleOnly((current) => !current)}
          className={`rounded-md border p-4 text-left transition-colors hover:bg-muted/40 ${missingCycleOnly ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "bg-background"}`}
        >
          <div className="text-xs text-muted-foreground">Без цикла</div>
          <div className="mt-1 text-2xl font-semibold">{summary.withoutCycle}</div>
        </button>
      </div>

      <div className="mb-3 grid gap-3 lg:grid-cols-[1fr_220px_220px_190px_auto]">
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Поиск</span>
          <div className="relative">
            <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-10"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Название или идентификатор"
            />
          </div>
        </label>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Проектник</span>
          <Select value={managerFilter} onChange={(event) => setManagerFilter(event.target.value)}>
            <option value="">Все проектники</option>
            {managers.map((manager) => (
              <option key={manager.id || manager.name} value={manager.id || manager.name}>{manager.name}</option>
            ))}
          </Select>
        </label>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Тип проекта</span>
          <Select
            value={typeFilter}
            onChange={(event) => {
              setTypeFilter(event.target.value as RedmineProjectType)
              setWithoutTypeOnly(false)
            }}
          >
            <option value="">Все типы</option>
            {PROJECT_TYPES.filter((type) => type.key).map((type) => (
              <option key={type.key} value={type.key}>{type.label}</option>
            ))}
          </Select>
        </label>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Срок</span>
          <Select value={deadlineFilter} onChange={(event) => setDeadlineFilter(event.target.value)}>
            <option value="">Все сроки</option>
            <option value="urgent">Срочные</option>
            <option value="burning">Горят</option>
            <option value="due_soon">Скоро срок</option>
            <option value="ok">Ок</option>
          </Select>
        </label>
        <div className="text-sm text-muted-foreground lg:self-end lg:pb-2">
          Показано {filteredItems.length} из {items.length}
        </div>
      </div>

      {activeFilterChips.length > 0 && (
        <div className="mb-6 flex flex-wrap items-center gap-2">
          {activeFilterChips.map((chip) => (
            <button
              key={chip.key}
              type="button"
              onClick={chip.onRemove}
              className="inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/5 py-1 pl-3 pr-2 text-xs font-medium text-primary hover:bg-primary/10"
            >
              {chip.label}
              <X className="h-3 w-3" />
            </button>
          ))}
        </div>
      )}

      <div className="mb-6 flex flex-wrap gap-3">
        <div className="inline-flex rounded-md border bg-muted p-1">
          <button type="button" className={`rounded px-3 py-1.5 text-sm ${viewMode === "status" ? "bg-background shadow-sm" : "text-muted-foreground"}`} onClick={() => setViewMode("status")}>
            По статусу
          </button>
          <button type="button" className={`rounded px-3 py-1.5 text-sm ${viewMode === "manager" ? "bg-background shadow-sm" : "text-muted-foreground"}`} onClick={() => setViewMode("manager")}>
            По проектнику
          </button>
        </div>
        <div className="inline-flex rounded-md border bg-muted p-1">
          {[
            ["lead", "Руководитель"],
            ["manager", "Проектник"],
            ["docs", "Документы"],
            ["compact", "Компактно"],
          ].map(([key, label]) => (
            <button
              key={key}
              type="button"
              className={`rounded px-3 py-1.5 text-sm ${cardMode === key ? "bg-background shadow-sm" : "text-muted-foreground"}`}
              onClick={() => setCardMode(key as typeof cardMode)}
            >
              {label}
            </button>
          ))}
        </div>
        {(statusFilter || managerFilter || typeFilter || deadlineFilter || searchQuery || withoutTypeOnly || missingCycleOnly) && (
          <Button
            variant="outline"
            onClick={() => {
              setStatusFilter("")
              setManagerFilter("")
              setTypeFilter("")
              setDeadlineFilter("")
              setSearchQuery("")
              setWithoutTypeOnly(false)
              setMissingCycleOnly(false)
            }}
          >
            Сбросить фильтры
          </Button>
        )}
      </div>

      {error && <Alert className="mb-6">{error}</Alert>}

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : viewMode === "status" ? (
        <div className="grid gap-6 xl:grid-cols-4">
          {STATUS_COLUMNS.map((column) => {
            const list = itemsByColumn[column.key]
            return (
              <section
                key={column.key}
                className={`min-w-0 rounded-md ${dragProjectId ? "bg-muted/30 p-2" : ""}`}
                onDragOver={(event) => {
                  event.preventDefault()
                  event.dataTransfer.dropEffect = "move"
                }}
                onDrop={() => handleDropToColumn(column.key)}
              >
                <div className="mb-3 flex items-center gap-2">
                  <span className={`h-3 w-3 rounded-full ${column.tone}`} />
                  <h2 className="font-semibold">{column.title}</h2>
                  <span className="text-sm text-muted-foreground">{list.length}</span>
                  <span className="ml-auto text-xs text-muted-foreground">{countsByColumn[column.key]}</span>
                </div>
                <div className="space-y-3">
                  {list.length === 0 ? (
                    <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                      Нет проектов
                    </div>
                  ) : (
                    list.map(renderProject)
                  )}
                </div>
              </section>
            )
          })}
        </div>
      ) : (
        <div className="space-y-8">
          {itemsByManager.map(({ manager, list }) => (
            <section key={manager} className="space-y-3">
              <div className="flex items-center gap-2">
                <UserRound className="h-4 w-4 text-muted-foreground" />
                <h2 className="font-semibold">{manager}</h2>
                <span className="text-sm text-muted-foreground">{list.length}</span>
              </div>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {list.map(renderProject)}
              </div>
            </section>
          ))}
          {itemsByManager.length === 0 && (
            <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
              По текущим фильтрам проектов нет.
            </div>
          )}
        </div>
      )}

      <Dialog open={!!deleteEventTarget} onOpenChange={(open) => !open && setDeleteEventTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Удалить контрольную дату?</DialogTitle>
            <DialogDescription>
              {deleteEventTarget && `"${deleteEventTarget.event.title}" на ${deleteEventTarget.event.due_date}`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteEventTarget(null)}
              disabled={!!deleteEventTarget && savingProjectId === deleteEventTarget.item.project_id}
            >
              Отмена
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={confirmEventDelete}
              disabled={!!deleteEventTarget && savingProjectId === deleteEventTarget.item.project_id}
            >
              {deleteEventTarget && savingProjectId === deleteEventTarget.item.project_id ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Удалить"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

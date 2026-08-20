"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeft, CalendarDays, ExternalLink, FileCheck, FileText, Loader2, RefreshCw, Trash2, UserRound } from "lucide-react"

import {
  redmineAPI,
  RedmineIssue,
  RedmineProjectControlEvent,
  RedmineProjectDashboardItem,
  RedmineProjectDocumentsResponse,
  RedmineProjectGroup,
  RedmineProjectType,
} from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { eventSentActionLabel, groupForColumn, groupKeyForItem, nextMonthDate, PROJECT_TYPES, projectTypeLabel } from "../shared"

function eventStatusLabel(event: RedmineProjectControlEvent) {
  if (event.status === "sent") return "Отправлено"
  if (event.status === "skipped") return "Пропущено"
  return "План"
}

export default function RedmineProjectPage() {
  const params = useParams<{ project: string }>()
  const projectId = params.project
  const [project, setProject] = useState<RedmineProjectDashboardItem | null>(null)
  const [groups, setGroups] = useState<RedmineProjectGroup[]>([])
  const [managers, setManagers] = useState<string[]>([])
  const [issues, setIssues] = useState<RedmineIssue[]>([])
  const [documents, setDocuments] = useState<RedmineProjectDocumentsResponse | null>(null)
  const [controlEvents, setControlEvents] = useState<RedmineProjectControlEvent[]>([])
  // Guards against a slow loadControlEvents GET resolving after a faster
  // generate/mark-sent/delete mutation and clobbering it with stale data.
  const controlEventsRequestId = useRef(0)
  const [cycleDate, setCycleDate] = useState(nextMonthDate())
  const [deleteEventTarget, setDeleteEventTarget] = useState<RedmineProjectControlEvent | null>(null)
  const [loading, setLoading] = useState(true)
  const [tabLoading, setTabLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  const loadProject = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const response = await redmineAPI.getDashboard()
      setGroups(response.groups || [])
      setManagers([...(response.managers || [])].sort((left, right) => left.localeCompare(right, "ru")))
      setProject((response.data || []).find((item) => item.project_id === projectId) || null)
    } catch (err: unknown) {
      console.error("Failed to load Redmine project:", err)
      setError(err instanceof Error ? err.message : "Не удалось загрузить проект")
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    loadProject()
  }, [loadProject])

  const loadIssues = useCallback(async () => {
    setTabLoading(true)
    setError("")
    try {
      const response = await redmineAPI.getProjectIssues(projectId)
      setIssues(response.data || [])
    } catch (err: unknown) {
      console.error("Failed to load Redmine issues:", err)
      setError(err instanceof Error ? err.message : "Не удалось загрузить задачи")
    } finally {
      setTabLoading(false)
    }
  }, [projectId])

  const loadDocuments = useCallback(async () => {
    setTabLoading(true)
    setError("")
    try {
      const response = await redmineAPI.getProjectDocuments(projectId)
      setDocuments(response)
    } catch (err: unknown) {
      console.error("Failed to load Redmine documents:", err)
      setError(err instanceof Error ? err.message : "Не удалось загрузить документы")
    } finally {
      setTabLoading(false)
    }
  }, [projectId])

  const loadControlEvents = useCallback(async () => {
    const requestId = ++controlEventsRequestId.current
    setTabLoading(true)
    setError("")
    try {
      const response = await redmineAPI.getProjectControlEvents(projectId)
      if (requestId === controlEventsRequestId.current) setControlEvents(response.data || [])
    } catch (err: unknown) {
      console.error("Failed to load Redmine control events:", err)
      if (requestId === controlEventsRequestId.current) {
        setError(err instanceof Error ? err.message : "Не удалось загрузить контрольные даты")
      }
    } finally {
      if (requestId === controlEventsRequestId.current) setTabLoading(false)
    }
  }, [projectId])

  const currentStatus = useMemo(() => project ? groupKeyForItem(project) : "unknown", [project])

  const updateStatus = async (nextKey: string) => {
    if (!project) return
    const group = groupForColumn(groups, nextKey)
    const groupId = group?.id || ""
    setSaving(true)
    setError("")
    try {
      await redmineAPI.assignProjectGroup(project.project_id, groupId)
      setProject({
        ...project,
        group_id: groupId,
        group_name: group?.name || "",
        group_color: group?.color || "",
        group_assigned_manually: true,
      })
    } catch (err: unknown) {
      console.error("Failed to update project status:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить статус")
    } finally {
      setSaving(false)
    }
  }

  const updateManager = async (managerName: string) => {
    if (!project) return
    setSaving(true)
    setError("")
    try {
      await redmineAPI.assignProjectManager(project.project_id, { manager_name: managerName })
      setProject({
        ...project,
        manual_manager_name: managerName,
        manual_manager_id: "",
        effective_manager_name: managerName || project.inferred_manager_name,
        effective_manager_id: managerName ? "" : project.inferred_manager_id,
      })
    } catch (err: unknown) {
      console.error("Failed to update project manager:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить проектника")
    } finally {
      setSaving(false)
    }
  }

  const updateProjectType = async (projectType: RedmineProjectType) => {
    if (!project) return
    setSaving(true)
    setError("")
    try {
      await redmineAPI.updateProjectOperations(project.project_id, { project_type: projectType })
      setProject({ ...project, project_type: projectType })
    } catch (err: unknown) {
      console.error("Failed to update project type:", err)
      setError(err instanceof Error ? err.message : "Не удалось изменить тип проекта")
    } finally {
      setSaving(false)
    }
  }

  const generateCycle = async () => {
    if (!project || !project.project_type) {
      setError("Сначала задайте тип проекта")
      return
    }
    setSaving(true)
    setError("")
    try {
      controlEventsRequestId.current += 1
      const response = await redmineAPI.generateProjectCycle(project.project_id, {
        project_type: project.project_type,
        report_date: cycleDate,
      })
      setControlEvents(response.data || [])
      await loadProject()
    } catch (err: unknown) {
      console.error("Failed to generate control events:", err)
      setError(err instanceof Error ? err.message : "Не удалось создать цикл")
    } finally {
      setSaving(false)
    }
  }

  const markEventSent = async (event: RedmineProjectControlEvent) => {
    if (!project) return
    setSaving(true)
    setError("")
    try {
      controlEventsRequestId.current += 1
      const response = await redmineAPI.markControlEventSent(project.project_id, event.id, {
        sent_by: project.effective_manager_name,
      })
      setControlEvents(response.data || [])
      await loadProject()
    } catch (err: unknown) {
      console.error("Failed to mark control event sent:", err)
      setError(err instanceof Error ? err.message : "Не удалось отметить отправку")
    } finally {
      setSaving(false)
    }
  }

  const confirmEventDelete = async () => {
    if (!project || !deleteEventTarget) return
    setSaving(true)
    setError("")
    try {
      controlEventsRequestId.current += 1
      const response = await redmineAPI.deleteControlEvent(project.project_id, deleteEventTarget.id)
      setControlEvents(response.data || [])
      await loadProject()
      setDeleteEventTarget(null)
    } catch (err: unknown) {
      console.error("Failed to delete control event:", err)
      setError(err instanceof Error ? err.message : "Не удалось удалить контрольную дату")
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (!project) {
    return (
      <div className="container mx-auto max-w-4xl px-4 py-8">
        <Link href="/redmine">
          <Button variant="outline">
            <ArrowLeft className="mr-2 h-4 w-4" />
            К дашборду
          </Button>
        </Link>
        <div className="mt-8 rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
          Проект не найден в локальном зеркале Redmine.
        </div>
      </div>
    )
  }

  const effectiveManager = project.effective_manager_name || "Не назначен"
  const localDocuments = documents?.local_documents || []
  const redmineFiles = documents?.files || []

  return (
    <div className="container mx-auto max-w-5xl px-4 py-8">
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-4">
          <Link href="/redmine">
            <Button variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              К дашборду
            </Button>
          </Link>
          <div>
            <h1 className="text-3xl font-bold">{project.name}</h1>
            <p className="text-sm text-muted-foreground">{project.identifier}</p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={loadProject} disabled={saving}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Обновить
          </Button>
          {project.url && (
            <a href={project.url} target="_blank" rel="noreferrer">
              <Button>
                <ExternalLink className="mr-2 h-4 w-4" />
                Redmine
              </Button>
            </a>
          )}
        </div>
      </div>

      {error && <Alert className="mb-6">{error}</Alert>}

      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Обзор</TabsTrigger>
          <TabsTrigger value="issues" onClick={loadIssues}>Задачи</TabsTrigger>
          <TabsTrigger value="documents" onClick={loadDocuments}>Документы</TabsTrigger>
          <TabsTrigger value="control" onClick={loadControlEvents}>Контрольные даты</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <div className="grid gap-6 lg:grid-cols-[320px_1fr]">
        <aside className="space-y-4 rounded-md border bg-background p-4">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Проектник</div>
            <div className="mt-2 flex items-center gap-2 text-sm">
              <UserRound className="h-4 w-4 text-muted-foreground" />
              {effectiveManager}
            </div>
          </div>

          <label className="block space-y-1">
            <span className="text-xs font-medium text-muted-foreground">Задать проектника вручную</span>
            <Select
              value={project.manual_manager_name || ""}
              onChange={(event) => updateManager(event.target.value)}
              disabled={saving}
            >
              <option value="">Использовать авто-вывод</option>
              {managers.map((manager) => (
                <option key={manager} value={manager}>{manager}</option>
              ))}
            </Select>
          </label>

          <label className="block space-y-1">
            <span className="text-xs font-medium text-muted-foreground">Статус в work-app</span>
            <Select
              value={currentStatus}
              onChange={(event) => updateStatus(event.target.value)}
              disabled={saving}
            >
              <option value="active">Активный</option>
              <option value="pause">Пауза</option>
              <option value="done">Завершенный</option>
              <option value="unknown">Не разобран</option>
            </Select>
          </label>

          <label className="block space-y-1">
            <span className="text-xs font-medium text-muted-foreground">Тип проекта</span>
            <Select
              value={project.project_type}
              onChange={(event) => updateProjectType(event.target.value as RedmineProjectType)}
              disabled={saving}
            >
              {PROJECT_TYPES.map((type) => (
                <option key={type.key || "none"} value={type.key}>{type.label}</option>
              ))}
            </Select>
          </label>
        </aside>

        <main className="space-y-6">
          <section className="rounded-md border bg-background p-4">
            <h2 className="mb-3 text-lg font-semibold">Описание Redmine</h2>
            {project.description ? (
              <div className="whitespace-pre-wrap text-sm leading-6">{project.description}</div>
            ) : (
              <div className="text-sm text-muted-foreground">Описание в Redmine пустое.</div>
            )}
          </section>

          <section className="rounded-md border bg-background p-4">
            <h2 className="mb-4 text-lg font-semibold">Данные проекта</h2>
            <dl className="grid gap-3 text-sm md:grid-cols-2">
              <div>
                <dt className="text-muted-foreground">Redmine ID</dt>
                <dd className="font-medium">{project.project_id}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Identifier</dt>
                <dd className="font-medium">{project.identifier}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Статус Redmine</dt>
                <dd className="font-medium">{project.status}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Видимость</dt>
                <dd className="font-medium">{project.is_public ? "Публичный" : "Закрытый"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Авто-проектник по задачам</dt>
                <dd className="font-medium">{project.inferred_manager_name || "Не определен"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Ручной проектник</dt>
                <dd className="font-medium">{project.manual_manager_name || "Не задан"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Задача для авто-вывода</dt>
                <dd className="font-medium">{project.inferred_issue_id || "Нет"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Тип проекта</dt>
                <dd className="font-medium">{projectTypeLabel(project.project_type)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Ближайшая контрольная дата</dt>
                <dd className="font-medium">{project.next_control_event ? `${project.next_control_event.title}: ${project.next_control_event.due_date}` : "Не задана"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Последняя синхронизация</dt>
                <dd className="font-medium">{new Date(project.synced_at).toLocaleString("ru-RU")}</dd>
              </div>
            </dl>
          </section>
        </main>
      </div>
        </TabsContent>

        <TabsContent value="issues">
          <section className="rounded-md border bg-background p-4">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">Активные задачи</h2>
              <Button variant="outline" onClick={loadIssues} disabled={tabLoading}>
                {tabLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
                Обновить
              </Button>
            </div>
            {tabLoading && issues.length === 0 ? (
              <div className="flex justify-center py-10">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : issues.length === 0 ? (
              <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                Активных задач нет.
              </div>
            ) : (
              <div className="divide-y">
                {issues.map((issue) => (
                  <a
                    key={issue.id}
                    href={`${project.url?.split("/projects/")[0] || ""}/issues/${issue.id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="grid gap-2 py-3 text-sm hover:bg-muted/40 md:grid-cols-[90px_1fr_180px_160px]"
                  >
                    <div className="font-medium">#{issue.id}</div>
                    <div>
                      <div className="font-medium">{issue.subject}</div>
                      <div className="text-xs text-muted-foreground">{issue.status} · {issue.priority}</div>
                    </div>
                    <div className="text-muted-foreground">{issue.assigned_to || "Не назначена"}</div>
                    <div className="text-muted-foreground">{issue.updated_on ? new Date(issue.updated_on).toLocaleDateString("ru-RU") : ""}</div>
                  </a>
                ))}
              </div>
            )}
          </section>
        </TabsContent>

        <TabsContent value="control">
          <section className="space-y-6">
            <div className="rounded-md border bg-background p-4">
              <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <h2 className="text-lg font-semibold">Цикл проекта</h2>
                  <p className="text-sm text-muted-foreground">
                    Тип: {projectTypeLabel(project.project_type)}
                  </p>
                </div>
                <Button variant="outline" onClick={loadControlEvents} disabled={tabLoading || saving}>
                  {tabLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
                  Обновить
                </Button>
              </div>

              <div className="grid gap-3 md:grid-cols-[220px_auto_1fr]">
                <Input type="date" value={cycleDate} onChange={(event) => setCycleDate(event.target.value)} />
                <Button onClick={generateCycle} disabled={saving || !project.project_type}>
                  {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <CalendarDays className="mr-2 h-4 w-4" />}
                  Создать цикл от ОД
                </Button>
                <div className="text-sm text-muted-foreground md:self-center">
                  Для рекламы создаются КС 1/2/3 и ОД, для SEO и техподдержки - ОД.
                </div>
              </div>
            </div>

            <div className="rounded-md border bg-background p-4">
              <h2 className="mb-4 text-lg font-semibold">Контрольные события</h2>
              {tabLoading && controlEvents.length === 0 ? (
                <div className="flex justify-center py-10">
                  <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
              ) : controlEvents.length === 0 ? (
                <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  Контрольные даты пока не настроены.
                </div>
              ) : (
                <div className="divide-y">
                  {controlEvents.map((event) => (
                    <div key={event.id} className="grid gap-3 py-3 text-sm md:grid-cols-[120px_1fr_120px_200px] md:items-center">
                      <div className="font-medium">{event.due_date}</div>
                      <div>
                        <div className="font-medium">{event.title}</div>
                        <div className="text-xs text-muted-foreground">{event.event_type === "control_cut" ? "Контрольный срез" : "Отчетная дата"}</div>
                      </div>
                      <div className={event.status === "sent" ? "text-success" : "text-muted-foreground"}>
                        {eventStatusLabel(event)}
                      </div>
                      {event.status === "planned" ? (
                        <div className="flex gap-2">
                          <Button size="sm" variant="outline" disabled={saving} onClick={() => markEventSent(event)}>
                            {eventSentActionLabel(event)}
                          </Button>
                          <Button size="sm" variant="outline" disabled={saving} aria-label="Удалить контрольную дату" onClick={() => setDeleteEventTarget(event)}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      ) : (
                        <div className="text-xs text-muted-foreground">
                          {event.sent_at ? new Date(event.sent_at).toLocaleString("ru-RU") : ""}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>
        </TabsContent>

        <TabsContent value="documents">
          <section className="space-y-6">
            <div className="rounded-md border bg-background p-4">
              <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <h2 className="text-lg font-semibold">Документы work-app</h2>
                  <p className="text-sm text-muted-foreground">
                    {documents?.customer_id ? `Контрагент: ${documents.customer_name}` : "Проект не привязан к контрагенту work-app"}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={loadDocuments} disabled={tabLoading}>
                    {tabLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
                    Обновить
                  </Button>
                  {documents?.customer_id && (
                    <Link href={`/${documents.customer_id}`}>
                      <Button>
                        <FileText className="mr-2 h-4 w-4" />
                        Создать счет/акт
                      </Button>
                    </Link>
                  )}
                </div>
              </div>

              {!documents ? (
                <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  Откройте вкладку или нажмите обновить, чтобы загрузить документы.
                </div>
              ) : localDocuments.length === 0 ? (
                <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  Локальных счетов и актов пока нет.
                </div>
              ) : (
                <div className="divide-y">
                  {localDocuments.map((doc) => {
                    const Icon = doc.type === "invoice" ? FileText : FileCheck
                    return (
                      <Link key={`${doc.type}-${doc.id}`} href={doc.url} className="grid gap-2 py-3 text-sm hover:bg-muted/40 md:grid-cols-[110px_1fr_160px_150px]">
                        <div className="flex items-center gap-2 font-medium">
                          <Icon className="h-4 w-4 text-muted-foreground" />
                          {doc.type === "invoice" ? "Счет" : "Акт"} № {doc.number}
                        </div>
                        <div className="text-muted-foreground">Договор: {doc.contract_number}</div>
                        <div>{doc.date}</div>
                        <div className={doc.uploaded_status === "uploaded" ? "text-success" : "text-muted-foreground"}>
                          {doc.uploaded_status === "uploaded" ? "В Redmine" : "Не отправлен"}
                        </div>
                      </Link>
                    )
                  })}
                </div>
              )}
            </div>

            <div className="rounded-md border bg-background p-4">
              <h2 className="mb-2 text-lg font-semibold">Файлы проекта Redmine</h2>
              <p className="mb-4 text-sm text-muted-foreground">
                PDF из work-app отправляются в стандартный раздел Redmine Files.
              </p>
              {!documents ? (
                <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  Файлы Redmine еще не загружены.
                </div>
              ) : redmineFiles.length === 0 ? (
                <div className="rounded-md border border-dashed px-4 py-8 text-sm text-muted-foreground">
                  В файлах проекта Redmine пока ничего нет.
                </div>
              ) : (
                <div className="divide-y">
                  {redmineFiles.map((file) => (
                    <a
                      key={file.id}
                      href={file.content_url}
                      target="_blank"
                      rel="noreferrer"
                      className="grid gap-2 py-3 text-sm hover:bg-muted/40 md:grid-cols-[1fr_140px_180px]"
                    >
                      <div>
                        <div className="font-medium">{file.filename}</div>
                        <div className="text-xs text-muted-foreground">{file.description || file.content_type}</div>
                      </div>
                      <div className="text-muted-foreground">{Math.round(file.filesize / 1024)} КБ</div>
                      <div className="text-muted-foreground">{file.created_on ? new Date(file.created_on).toLocaleDateString("ru-RU") : ""}</div>
                    </a>
                  ))}
                </div>
              )}
            </div>
          </section>
        </TabsContent>
      </Tabs>

      <Dialog open={!!deleteEventTarget} onOpenChange={(open) => !open && setDeleteEventTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Удалить контрольную дату?</DialogTitle>
            <DialogDescription>
              {deleteEventTarget && `"${deleteEventTarget.title}" на ${deleteEventTarget.due_date}`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleteEventTarget(null)} disabled={saving}>
              Отмена
            </Button>
            <Button type="button" variant="destructive" onClick={confirmEventDelete} disabled={saving}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : "Удалить"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

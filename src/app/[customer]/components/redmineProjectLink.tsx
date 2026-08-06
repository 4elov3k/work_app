"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { ExternalLink, Loader2, RefreshCw, Settings } from "lucide-react"

import { customersAPI, redmineAPI, RedmineProject, RedmineProjectLink } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"

export default function RedmineProjectLinkPanel({ customerId }: { customerId: string }) {
  const [projects, setProjects] = useState<RedmineProject[]>([])
  const [link, setLink] = useState<RedmineProjectLink | null>(null)
  const [selectedProjectId, setSelectedProjectId] = useState("")
  const [search, setSearch] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [manageOpen, setManageOpen] = useState(false)

  const selectedProject = useMemo(
    () => projects.find((project) => String(project.id) === selectedProjectId),
    [projects, selectedProjectId]
  )

  const filteredProjects = useMemo(() => {
    const needle = search.trim().toLowerCase()
    if (!needle) return projects

    return projects.filter((project) => (
      project.name.toLowerCase().includes(needle) ||
      project.identifier.toLowerCase().includes(needle)
    ))
  }, [projects, search])

  const formatError = (err: unknown) => {
    const message = err instanceof Error ? err.message : "Не удалось выполнить запрос"
    if (message.includes("HTTP 404")) {
      return "Redmine endpoints не найдены в backend. Перезапустите backend с новой сборкой и примените миграцию 011."
    }
    if (message.includes("HTTP 502")) {
      return "Backend не смог получить проекты из Redmine. Проверьте REDMINE_URL и REDMINE_API в окружении backend."
    }
    return message
  }

  const loadData = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [projectResponse, linkResponse] = await Promise.all([
        redmineAPI.getProjects("", 500),
        customersAPI.getRedmineProject(customerId),
      ])
      setProjects(projectResponse.data || [])
      setLink(linkResponse.data)
      if (linkResponse.data) {
        setSelectedProjectId(linkResponse.data.redmine_project_id)
      }
    } catch (err: unknown) {
      console.error("Failed to load Redmine project data:", err)
      setError(formatError(err))
    } finally {
      setLoading(false)
    }
  }, [customerId])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleSave = async () => {
    if (!selectedProject) {
      setError("Выберите проект Redmine")
      return
    }

    setSaving(true)
    setError("")
    try {
      const response = await customersAPI.linkRedmineProject(customerId, {
        project_id: String(selectedProject.id),
        project_identifier: selectedProject.identifier,
        project_name: selectedProject.name,
      })
      setLink(response.data)
      setManageOpen(false)
    } catch (err: unknown) {
      console.error("Failed to link Redmine project:", err)
      setError(formatError(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mt-4 border-t pt-4">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h3 className="text-base font-semibold">Redmine project</h3>
            {link ? (
              <Badge variant="secondary">привязан</Badge>
            ) : (
              <Badge variant="outline">не привязан</Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {link ? link.redmine_project_name : "Выберите карточку проекта Redmine для этого контрагента"}
          </p>
        </div>

        {link && (
          <div className="flex items-center gap-2">
            {link.redmine_url && (
              <a
                href={link.redmine_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center text-sm text-primary hover:underline"
              >
                Открыть в Redmine
                <ExternalLink className="ml-1 h-4 w-4" />
              </a>
            )}
            <Button
              type="button"
              variant="outline"
              size="icon"
              onClick={() => setManageOpen((value) => !value)}
              aria-label="Настроить привязку Redmine"
            >
              <Settings className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>

      {(!link || manageOpen) && (
        <div className="mt-4 grid gap-3 md:grid-cols-[minmax(180px,260px)_1fr_auto]">
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Поиск проекта"
            disabled={saving}
          />
          <Select
            value={selectedProjectId}
            onChange={(event) => setSelectedProjectId(event.target.value)}
            disabled={loading || saving}
          >
            <option value="">Выберите проект</option>
            {filteredProjects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name} ({project.identifier})
              </option>
            ))}
          </Select>
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={loadData} disabled={loading || saving} aria-label="Обновить проекты">
              <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            </Button>
            <Button type="button" onClick={handleSave} disabled={loading || saving || !selectedProjectId}>
              {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              {link ? "Сменить" : "Привязать"}
            </Button>
          </div>
        </div>
      )}

      {error && <Alert className="mt-3">{error}</Alert>}
    </div>
  )
}

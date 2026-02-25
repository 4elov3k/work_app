"use client"
import React, { useCallback, useEffect, useState } from "react"
import { Calendar, FileText, Loader2, Plus, Trash2 } from "lucide-react"
import Link from "next/link"

import { Contract, contractsAPI } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"

const TOPICS = [
  "Продвижение сео",
  "Продвижение контекст",
  "Сео + контекст",
  "Техподдержка",
  "Юр услуги",
  "Разработка",
  "Соц сети",
  "Дизайн",
  "Отзывы",
]

export default function ContractsList({ slug }: { slug: string }) {
  const [contracts, setContracts] = useState<Contract[]>([])
  const [loading, setLoading] = useState(true)
  const [isOpen, setIsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [number, setNumber] = useState("")
  const [manualNumber, setManualNumber] = useState(false)
  const [autoLoading, setAutoLoading] = useState(false)
  const [startDate, setStartDate] = useState("")
  const [status, setStatus] = useState("active")
  const [topic, setTopic] = useState(TOPICS[0])
  const [error, setError] = useState("")
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Contract | null>(null)

  const loadContracts = useCallback(() => {
    contractsAPI
      .getByCustomer(slug)
      .then((response) => setContracts(response.data || []))
      .catch((err) => {
        console.error("Failed to load contracts:", err)
        setContracts([])
      })
      .finally(() => setLoading(false))
  }, [slug])

  useEffect(() => {
    loadContracts()
  }, [loadContracts])

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      if (!number) {
        setError("Номер договора обязателен")
        return
      }
      await contractsAPI.create({
        customer_id: slug,
        number,
        status,
        topic,
        start_date: startDate || undefined,
        currency: "RUB",
      })
      setNumber("")
      setStartDate("")
      setStatus("active")
      setTopic(TOPICS[0])
      setIsOpen(false)
      loadContracts()
    } catch (err: unknown) {
      console.error("Failed to create contract:", err)
      const message = err instanceof Error ? err.message : "Ошибка при создании договора"
      if (message.includes("HTTP 409") || message.includes("already exists")) {
        setError("Договор с таким номером уже существует")
      } else {
        setError(message)
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setSubmitting(true)
    setError("")
    try {
      await contractsAPI.delete(deleteTarget.id)
      setDeleteOpen(false)
      setDeleteTarget(null)
      loadContracts()
    } catch (err: unknown) {
      console.error("Failed to delete contract:", err)
      const message = err instanceof Error ? err.message : "Ошибка при удалении договора"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  const loadNextNumber = useCallback(async () => {
    setAutoLoading(true)
    try {
      const res = await contractsAPI.getNextNumber(slug)
      setNumber(res.number)
    } catch (err: unknown) {
      console.error("Failed to load next contract number:", err)
      const message = err instanceof Error ? err.message : "Не удалось получить номер"
      setError(message)
    } finally {
      setAutoLoading(false)
    }
  }, [slug])

  const handleOpenChange = (value: boolean) => {
    setIsOpen(value)
    if (value) {
      setError("")
      if (!manualNumber) {
        loadNextNumber()
      }
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div>
          <h3 className="text-lg font-semibold">Договоры</h3>
          <p className="text-sm text-muted-foreground">Всего: {contracts.length}</p>
        </div>

        <Dialog open={isOpen} onOpenChange={handleOpenChange}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Добавить договор
            </Button>
          </DialogTrigger>
          <DialogContent>
            <form onSubmit={handleCreate}>
              <DialogHeader>
                <DialogTitle>Создать договор</DialogTitle>
                <DialogDescription>Заполните данные договора</DialogDescription>
              </DialogHeader>
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded text-sm">
                  {error}
                </div>
              )}
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <label htmlFor="contractNumber" className="text-sm font-medium">
                    Номер договора
                  </label>
                  <Input
                    id="contractNumber"
                    placeholder="D-001"
                    value={number}
                    onChange={(e) => setNumber(e.target.value)}
                    required
                    disabled={!manualNumber}
                  />
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={manualNumber}
                      onChange={(e) => {
                        setManualNumber(e.target.checked)
                        if (!e.target.checked) {
                          loadNextNumber()
                        }
                      }}
                    />
                    Ввести вручную
                    {autoLoading && <Loader2 className="h-4 w-4 animate-spin" />}
                  </label>
                </div>
                <div className="space-y-2">
                  <label htmlFor="contractDate" className="text-sm font-medium">
                    Дата договора
                  </label>
                  <Input
                    id="contractDate"
                    type="date"
                    value={startDate}
                    onChange={(e) => setStartDate(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="contractStatus" className="text-sm font-medium">
                    Статус
                  </label>
                  <select
                    id="contractStatus"
                    className="w-full border rounded-md px-3 py-2 text-sm bg-background"
                    value={status}
                    onChange={(e) => setStatus(e.target.value)}
                  >
                    <option value="active">Не архив</option>
                    <option value="archived">Архив</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <label htmlFor="contractTopic" className="text-sm font-medium">
                    Тематика
                  </label>
                  <select
                    id="contractTopic"
                    className="w-full border rounded-md px-3 py-2 text-sm bg-background"
                    value={topic}
                    onChange={(e) => setTopic(e.target.value)}
                  >
                    {TOPICS.map((t) => (
                      <option key={t} value={t}>
                        {t}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setIsOpen(false)} disabled={submitting}>
                  Отмена
                </Button>
                <Button type="submit" disabled={submitting}>
                  {submitting ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Создание...
                    </>
                  ) : (
                    "Создать"
                  )}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {loading ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[1, 2].map((i) => (
            <Card key={i} className="animate-pulse">
              <CardHeader>
                <div className="h-4 bg-muted rounded w-1/3 mb-2"></div>
                <div className="h-3 bg-muted rounded w-1/4"></div>
              </CardHeader>
            </Card>
          ))}
        </div>
      ) : contracts.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <FileText className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">Нет договоров</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {contracts.map((contract) => (
            <Link key={contract.id} href={`/${slug}/contracts/${contract.id}`} className="block">
              <Card className="hover:shadow-lg transition-shadow h-full cursor-pointer">
                <CardHeader>
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <FileText className="h-5 w-5 text-muted-foreground" />
                      <Badge variant={contract.status === "archived" ? "secondary" : "outline"}>
                        {contract.status === "archived" ? "Архив" : "Активен"}
                      </Badge>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={(e) => {
                        e.preventDefault()
                        setDeleteTarget(contract)
                        setDeleteOpen(true)
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                  <CardTitle className="text-xl">№ {contract.number}</CardTitle>
                  <CardDescription className="flex items-center gap-1">
                    <Calendar className="h-3 w-3" />
                    {contract.start_date || "Без даты"}
                  </CardDescription>
                  <CardDescription>Тематика: {contract.topic}</CardDescription>
                </CardHeader>
              </Card>
            </Link>
          ))}
        </div>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Удалить договор?</DialogTitle>
            <DialogDescription>Это действие нельзя отменить.</DialogDescription>
          </DialogHeader>
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded text-sm">
              {error}
            </div>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleteOpen(false)} disabled={submitting}>
              Отмена
            </Button>
            <Button type="button" variant="destructive" onClick={handleDelete} disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Удаление...
                </>
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

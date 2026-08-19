"use client"
import React, { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { Plus, FileText, FileCheck, Calendar, Loader2, RefreshCw, Search, Trash2 } from "lucide-react"

import {
  Invoice,
  Act,
  Contract,
  Service,
  RedmineDocumentStatus,
  invoicesAPI,
  actsAPI,
  contractsAPI,
  servicesAPI,
  customersAPI,
} from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
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

const CONFIG = {
  invoice: {
    title: "Счета",
    emptyLabel: "Нет счетов",
    addButton: "Добавить счет",
    createTitle: "Создать новый счет",
    createDescription: "Заполните данные для создания счета",
    numberLabel: "Номер счета",
    dateLabel: "Дата счета",
    badgeLabel: "Счет",
    badgeVariant: "outline" as const,
    duplicateError: "Счет с таким номером уже существует для этого клиента",
    Icon: FileText,
  },
  act: {
    title: "Акты",
    emptyLabel: "Нет актов",
    addButton: "Добавить акт",
    createTitle: "Создать новый акт",
    createDescription: "Заполните данные для создания акта",
    numberLabel: "Номер акта",
    dateLabel: "Дата акта",
    badgeLabel: "Акт",
    badgeVariant: "secondary" as const,
    duplicateError: "Акт с таким номером уже существует для этого клиента",
    Icon: FileCheck,
  },
}

type DocumentListProps = {
  slug: string
  documentType: "invoice" | "act"
  fixedContractId?: string
}

export default function DocumentList({ slug, documentType, fixedContractId }: DocumentListProps) {
  const cfg = CONFIG[documentType]
  const [fetchItems, setFetchItems] = useState<(Invoice | Act)[]>([])
  const [contracts, setContracts] = useState<Contract[]>([])
  const [services, setServices] = useState<Service[]>([])
  const [redmineStatuses, setRedmineStatuses] = useState<Record<string, RedmineDocumentStatus>>({})
  const [loading, setLoading] = useState(true)
  const [isOpen, setIsOpen] = useState<boolean>(false)
  const [submitting, setSubmitting] = useState(false)
  const [number, setNumber] = useState<string>("")
  const [manualNumber, setManualNumber] = useState(false)
  const [autoLoading, setAutoLoading] = useState(false)
  const [date, setDate] = useState<string>("")
  const [selectedContractId, setSelectedContractId] = useState<string>(fixedContractId || "")
  const [archiveFilter, setArchiveFilter] = useState<"all" | "true" | "false">("all")
  const [search, setSearch] = useState("")
  const [serviceName, setServiceName] = useState<string>("")
  const [servicePrice, setServicePrice] = useState<string>("")
  const [selectedServiceId, setSelectedServiceId] = useState<string>("")
  const [error, setError] = useState<string>("")
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Invoice | Act | null>(null)
  const [sheetSyncing, setSheetSyncing] = useState(false)
  const [syncedWithSheet, setSyncedWithSheet] = useState(false)
  const [sheetNote, setSheetNote] = useState("")

  const loadDocuments = useCallback(() => {
    const loader =
      documentType === "invoice"
        ? invoicesAPI.getByCustomer(slug, fixedContractId || "", archiveFilter)
        : actsAPI.getByCustomer(slug, fixedContractId || "", archiveFilter)
    loader
      .then((response) => setFetchItems(response.data || []))
      .catch((err) => {
        console.error(`Failed to load ${documentType}:`, err)
        setFetchItems([])
      })
      .finally(() => setLoading(false))
  }, [archiveFilter, documentType, fixedContractId, slug])

  const loadRedmineStatuses = useCallback(() => {
    customersAPI
      .getRedmineDocumentStatuses(slug)
      .then((response) => {
        const next: Record<string, RedmineDocumentStatus> = {}
        for (const status of response.data || []) {
          if (status.document_type === documentType) {
            next[status.document_id] = status
          }
        }
        setRedmineStatuses(next)
      })
      .catch((err) => {
        console.error("Failed to load Redmine document statuses:", err)
        setRedmineStatuses({})
      })
  }, [documentType, slug])

  const loadContracts = useCallback(() => {
    if (fixedContractId) {
      setContracts([])
      setSelectedContractId(fixedContractId)
      return
    }
    contractsAPI
      .getByCustomer(slug)
      .then((response) => {
        const list = response.data || []
        setContracts(list)
        if (list.length > 0) {
          // Functional form so this doesn't need selectedContractId in its
          // closure — keeping it out of the deps below avoids loadContracts'
          // identity changing right after its own setState, which used to
          // re-trigger the whole mount effect (loadDocuments/loadRedmineStatuses/
          // loadServices too, not just this) a second time.
          setSelectedContractId((current) => current || list[0].id)
        }
      })
      .catch((err) => {
        console.error("Failed to load contracts:", err)
        setContracts([])
      })
  }, [fixedContractId, slug])

  const loadServices = useCallback(() => {
    servicesAPI
      .getAll(1, 1000)
      .then((response) => setServices(response.data || []))
      .catch((err) => {
        console.error("Failed to load services:", err)
        setServices([])
      })
  }, [])

  const loadNextNumber = useCallback(async (contractID: string) => {
    setAutoLoading(true)
    try {
      const res = await contractsAPI.getNextDocNumber(contractID, documentType)
      setNumber(res.number)
      setSyncedWithSheet(false)
      setSheetNote("")
    } catch (err: unknown) {
      console.error("Failed to load next number:", err)
      const message = err instanceof Error ? err.message : "Не удалось получить номер"
      setError(message)
    } finally {
      setAutoLoading(false)
    }
  }, [documentType])

  useEffect(() => {
    loadDocuments()
    loadRedmineStatuses()
    loadContracts()
    loadServices()
  }, [loadDocuments, loadRedmineStatuses, loadContracts, loadServices])

  useEffect(() => {
    if (!manualNumber && selectedContractId) {
      loadNextNumber(selectedContractId)
    }
  }, [selectedContractId, manualNumber, loadNextNumber])

  const handleSyncWithSheet = async () => {
    setSheetSyncing(true)
    setError("")
    setSheetNote("")
    try {
      const res = await actsAPI.getNextNumberFromSheet()
      setNumber(res.data.number)
      setManualNumber(true)
      setSyncedWithSheet(true)
      setSheetNote(`Номер ${res.data.number} взят из таблицы (строка ${res.data.row}). При создании акта строка будет дописана автоматически.`)
    } catch (err: unknown) {
      console.error("Failed to sync act number with sheet:", err)
      const message = err instanceof Error ? err.message : "Не удалось синхронизироваться с таблицей"
      setError(message.replace(/\s*\(HTTP \d+\)$/, ""))
    } finally {
      setSheetSyncing(false)
    }
  }

  const newDate = date.split("-").reverse().join(".")

  const filteredItems = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return fetchItems
    return fetchItems.filter(
      (item) =>
        item.number.toLowerCase().includes(query) ||
        item.contract_number.toLowerCase().includes(query)
    )
  }, [fetchItems, search])

  const handleOpenChange = (value: boolean) => {
    setIsOpen(value)
    if (value) {
      setError("")
      if (!manualNumber && selectedContractId) {
        loadNextNumber(selectedContractId)
      }
    }
  }

  const handleDeleteOpenChange = (value: boolean) => {
    setDeleteOpen(value)
    if (value) setError("")
  }

  const handleClickAdd = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      if (!selectedContractId) {
        setError("Выберите договор")
        return
      }
      if (!number) {
        setError("Номер документа обязателен")
        return
      }
      if (documentType === "invoice") {
        await invoicesAPI.create({
          contract_id: selectedContractId,
          customer_id: slug,
          number: number,
          date: newDate,
          service_ids: selectedServiceId ? [selectedServiceId] : undefined,
          services: selectedServiceId ? undefined : [{ name: serviceName, price: parseFloat(servicePrice) }],
        })
      } else {
        const actResponse = await actsAPI.create({
          contract_id: selectedContractId,
          customer_id: slug,
          number: number,
          date: newDate,
          service_ids: selectedServiceId ? [selectedServiceId] : undefined,
          services: selectedServiceId ? undefined : [{ name: serviceName, price: parseFloat(servicePrice) }],
        })

        if (syncedWithSheet) {
          try {
            await actsAPI.registerInSheet(actResponse.data.id)
          } catch (sheetErr: unknown) {
            console.error("Failed to register act in sheet:", sheetErr)
            const sheetMessage = sheetErr instanceof Error ? sheetErr.message : "неизвестная ошибка"
            window.alert(`Акт создан, но не удалось дописать его в таблицу: ${sheetMessage.replace(/\s*\(HTTP \d+\)$/, "")}`)
          }
        }
      }

      loadDocuments()

      setNumber("")
      setDate("")
      setServiceName("")
      setServicePrice("")
      setSelectedServiceId("")
      setSyncedWithSheet(false)
      setSheetNote("")
      loadRedmineStatuses()
      setIsOpen(false)
    } catch (err: unknown) {
      console.error(`Failed to create ${documentType}:`, err)
      const errorMessage = err instanceof Error ? err.message : "Ошибка при создании документа"
      if (errorMessage.includes("HTTP 409") || errorMessage.includes("already exists")) {
        setError(cfg.duplicateError)
      } else {
        setError(errorMessage)
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
      if (documentType === "invoice") {
        await invoicesAPI.delete(deleteTarget.id)
      } else {
        await actsAPI.delete(deleteTarget.id)
      }
      setDeleteOpen(false)
      setDeleteTarget(null)
      loadDocuments()
    } catch (err: unknown) {
      console.error(`Failed to delete ${documentType}:`, err)
      const message = err instanceof Error ? err.message : "Ошибка при удалении"
      setError(message.replace(/\s*\(HTTP \d+\)$/, ""))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h3 className="text-lg font-semibold">{cfg.title}</h3>
          <p className="text-sm text-muted-foreground">Всего: {fetchItems.length}</p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Поиск по номеру..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-44 pl-8"
            />
          </div>
          <Select
            className="w-36"
            value={archiveFilter}
            onChange={(e) => setArchiveFilter(e.target.value as "all" | "true" | "false")}
          >
            <option value="false">Активные</option>
            <option value="true">Архивные</option>
            <option value="all">Все</option>
          </Select>
          <Dialog open={isOpen} onOpenChange={handleOpenChange}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                {cfg.addButton}
              </Button>
            </DialogTrigger>
            <DialogContent>
            <form onSubmit={handleClickAdd}>
              <DialogHeader>
                <DialogTitle>{cfg.createTitle}</DialogTitle>
                <DialogDescription>{cfg.createDescription}</DialogDescription>
              </DialogHeader>
              {error && <Alert>{error}</Alert>}
              {documentType === "act" && (
                <div className="pt-2">
                  <Button type="button" variant="outline" size="sm" onClick={handleSyncWithSheet} disabled={sheetSyncing}>
                    {sheetSyncing ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        Синхронизация...
                      </>
                    ) : (
                      <>
                        <RefreshCw className="mr-2 h-4 w-4" />
                        Синхронизировать с таблицей
                      </>
                    )}
                  </Button>
                  {sheetNote && <p className="text-xs text-muted-foreground mt-2">{sheetNote}</p>}
                </div>
              )}
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <label htmlFor="number" className="text-sm font-medium">
                    {cfg.numberLabel}
                  </label>
                  <Input
                    id="number"
                    placeholder="3000"
                    value={number}
                    onChange={(e) => {
                      setNumber(e.target.value)
                      setSyncedWithSheet(false)
                      setSheetNote("")
                    }}
                    required
                    disabled={!manualNumber}
                  />
                  <label className="flex items-center gap-2 text-sm mt-1">
                    <input
                      type="checkbox"
                      checked={manualNumber}
                      onChange={(e) => {
                        setManualNumber(e.target.checked)
                        if (!e.target.checked && selectedContractId) {
                          loadNextNumber(selectedContractId)
                        }
                      }}
                    />
                    Ввести вручную
                    {autoLoading && <Loader2 className="h-4 w-4 animate-spin" />}
                  </label>
                </div>
                <div className="space-y-2">
                  <label htmlFor="date" className="text-sm font-medium">
                    {cfg.dateLabel}
                  </label>
                  <Input
                    id="date"
                    type="date"
                    value={date}
                    onChange={(e) => setDate(e.target.value)}
                    required
                  />
                </div>
                {!fixedContractId && (
                  <div className="space-y-2">
                    <label htmlFor="contractId" className="text-sm font-medium">
                      Договор
                    </label>
                    <Select
                      id="contractId"
                      value={selectedContractId}
                      onChange={(e) => setSelectedContractId(e.target.value)}
                      required
                    >
                      {contracts.length === 0 && <option value="">Нет договоров</option>}
                      {contracts.map((contract) => (
                        <option key={contract.id} value={contract.id}>
                          {contract.number} • {contract.topic} • {contract.status === "archived" ? "Архив" : "Активен"}
                        </option>
                      ))}
                    </Select>
                  </div>
                )}
                <div className="space-y-2">
                  <label htmlFor="serviceId" className="text-sm font-medium">
                    Готовая услуга
                  </label>
                  <Select
                    id="serviceId"
                    value={selectedServiceId}
                    onChange={(e) => setSelectedServiceId(e.target.value)}
                  >
                    <option value="">Не выбрано</option>
                    {services.filter((service) => !service.archived).map((service) => (
                      <option key={service.id} value={service.id}>
                        {service.name} • {service.price} ₽
                      </option>
                    ))}
                  </Select>
                </div>
                {!selectedServiceId && (
                  <>
                    <div className="space-y-2">
                      <label htmlFor="serviceName" className="text-sm font-medium">
                        Название услуги
                      </label>
                      <Input
                        id="serviceName"
                        placeholder="Консультация"
                        value={serviceName}
                        onChange={(e) => setServiceName(e.target.value)}
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <label htmlFor="servicePrice" className="text-sm font-medium">
                        Цена
                      </label>
                      <Input
                        id="servicePrice"
                        type="number"
                        placeholder="5000"
                        value={servicePrice}
                        onChange={(e) => setServicePrice(e.target.value)}
                        required
                      />
                    </div>
                  </>
                )}
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
      ) : fetchItems.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <cfg.Icon className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">{cfg.emptyLabel}</p>
          </CardContent>
        </Card>
      ) : filteredItems.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <Search className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">Ничего не найдено по запросу «{search}»</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filteredItems.map((item) => (
            <Card key={item.id} className="hover:shadow-lg transition-shadow h-full">
              <Link href={documentType === "act" ? `/${slug}/acts/${item.id}` : `/${slug}/${item.id}`}>
                <CardHeader>
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <cfg.Icon className="h-5 w-5 text-muted-foreground" />
                      <Badge variant={cfg.badgeVariant}>{cfg.badgeLabel}</Badge>
                      {item.archived && <Badge variant="secondary">Архив</Badge>}
                      {redmineStatuses[item.id]?.status === "uploaded" && <Badge variant="secondary">В Redmine</Badge>}
                      {redmineStatuses[item.id]?.status === "pending" && <Badge variant="outline">Отправляется</Badge>}
                      {redmineStatuses[item.id]?.status === "failed" && <Badge variant="destructive">Ошибка Redmine</Badge>}
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={(e) => {
                        e.preventDefault()
                        setDeleteTarget(item)
                        setDeleteOpen(true)
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                  <CardTitle className="text-xl">№ {item.number}</CardTitle>
                  <CardDescription className="flex items-center gap-1">
                    <Calendar className="h-3 w-3" />
                    {item.date}
                  </CardDescription>
                  <CardDescription>Договор: {item.contract_number}</CardDescription>
                </CardHeader>
              </Link>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={deleteOpen} onOpenChange={handleDeleteOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Удалить {cfg.badgeLabel.toLowerCase()}?</DialogTitle>
            <DialogDescription>Это действие нельзя отменить.</DialogDescription>
          </DialogHeader>
          {error && <Alert>{error}</Alert>}
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

"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { FileSpreadsheet, Loader2, Plus, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
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
import {
  ContractAppendix,
  ContractAppendixLineInput,
  ServiceCatalogSection,
  contractAppendicesAPI,
  serviceCatalogAPI,
} from "@/lib/api"

interface AppendixListProps {
  customerId: string
  contractId: string
}

interface CustomLine {
  title: string
  unit: string
  price: string
  qty: string
}

export default function AppendixList({ customerId, contractId }: AppendixListProps) {
  const [appendices, setAppendices] = useState<ContractAppendix[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  const [isOpen, setIsOpen] = useState(false)
  const [catalog, setCatalog] = useState<ServiceCatalogSection[]>([])
  const [catalogLoading, setCatalogLoading] = useState(false)
  const [number, setNumber] = useState("")
  const [date, setDate] = useState("")
  const [selected, setSelected] = useState<Record<string, string>>({}) // service_id -> qty
  const [customLines, setCustomLines] = useState<CustomLine[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState("")

  const loadAppendices = useCallback(() => {
    setLoading(true)
    contractAppendicesAPI
      .getByContract(contractId)
      .then((res) => {
        setAppendices(res.data || [])
        setError("")
      })
      .catch((err) => {
        console.error("Failed to load contract appendices:", err)
        setError("Не удалось загрузить приложения. Попробуйте обновить страницу.")
      })
      .finally(() => setLoading(false))
  }, [contractId])

  useEffect(() => {
    loadAppendices()
  }, [loadAppendices])

  const openDialog = () => {
    setFormError("")
    setSelected({})
    setCustomLines([])
    setDate(new Date().toLocaleDateString("ru-RU"))
    setCatalogLoading(true)
    Promise.all([contractAppendicesAPI.getNextNumber(contractId), serviceCatalogAPI.get()])
      .then(([nextNumber, catalogRes]) => {
        setNumber(nextNumber.number)
        setCatalog(catalogRes.data || [])
      })
      .catch((err) => {
        console.error("Failed to load catalog / next number:", err)
        setFormError("Не удалось загрузить каталог услуг.")
      })
      .finally(() => setCatalogLoading(false))
    setIsOpen(true)
  }

  const toggleItem = (serviceId: string, checked: boolean) => {
    setSelected((current) => {
      const next = { ...current }
      if (checked) {
        next[serviceId] = next[serviceId] || "1"
      } else {
        delete next[serviceId]
      }
      return next
    })
  }

  const setQty = (serviceId: string, qty: string) => {
    setSelected((current) => ({ ...current, [serviceId]: qty }))
  }

  const addCustomLine = () => {
    setCustomLines((current) => [...current, { title: "", unit: "услуга", price: "", qty: "1" }])
  }

  const updateCustomLine = (index: number, patch: Partial<CustomLine>) => {
    setCustomLines((current) => current.map((line, i) => (i === index ? { ...line, ...patch } : line)))
  }

  const removeCustomLine = (index: number) => {
    setCustomLines((current) => current.filter((_, i) => i !== index))
  }

  const catalogById = new Map(catalog.flatMap((section) => section.items.map((item) => [item.id, item])))

  const total = (() => {
    let sum = 0
    for (const [serviceId, qty] of Object.entries(selected)) {
      const item = catalogById.get(serviceId)
      if (item) sum += item.price * (parseFloat(qty) || 0)
    }
    for (const line of customLines) {
      sum += (parseFloat(line.price) || 0) * (parseFloat(line.qty) || 0)
    }
    return sum
  })()

  const handleSubmit = async () => {
    setFormError("")
    if (!number.trim()) {
      setFormError("Номер приложения обязателен")
      return
    }
    if (!date.trim()) {
      setFormError("Дата обязательна")
      return
    }
    const lines: ContractAppendixLineInput[] = []
    for (const [serviceId, qty] of Object.entries(selected)) {
      lines.push({ service_id: serviceId, qty: parseFloat(qty) || 1 })
    }
    for (const line of customLines) {
      if (!line.title.trim()) continue
      lines.push({
        title: line.title.trim(),
        unit: line.unit.trim() || "услуга",
        price: parseFloat(line.price) || 0,
        qty: parseFloat(line.qty) || 1,
      })
    }
    if (lines.length === 0) {
      setFormError("Выберите хотя бы одну позицию из каталога или добавьте свою строку")
      return
    }

    setSubmitting(true)
    try {
      await contractAppendicesAPI.create(contractId, { number, date, lines })
      setIsOpen(false)
      loadAppendices()
    } catch (err) {
      console.error("Failed to create contract appendix:", err)
      setFormError(err instanceof Error ? err.message : "Не удалось создать приложение")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold">Приложения к договору</h3>
        <Dialog open={isOpen} onOpenChange={setIsOpen}>
          <DialogTrigger asChild>
            <Button onClick={openDialog}>
              <Plus className="mr-2 h-4 w-4" />
              Создать приложение
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Новое приложение к договору</DialogTitle>
              <DialogDescription>
                Выберите позиции из каталога и/или добавьте свои строки. Цену и количество можно менять под конкретного клиента.
              </DialogDescription>
            </DialogHeader>

            {formError && (
              <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800">{formError}</div>
            )}

            <div className="grid grid-cols-2 gap-4 mb-2">
              <div>
                <label className="text-sm font-medium mb-1 block">Номер приложения</label>
                <Input value={number} onChange={(e) => setNumber(e.target.value)} placeholder="1" />
              </div>
              <div>
                <label className="text-sm font-medium mb-1 block">Дата</label>
                <Input value={date} onChange={(e) => setDate(e.target.value)} placeholder="ДД.ММ.ГГГГ" />
              </div>
            </div>

            {catalogLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <div className="space-y-4">
                {catalog.map((section) => (
                  <div key={section.section}>
                    <p className="text-sm font-semibold mb-2">{section.section}</p>
                    <div className="space-y-1">
                      {section.items.map((item) => (
                        <div key={item.id} className="flex items-start gap-2 text-sm">
                          <input
                            type="checkbox"
                            className="mt-1"
                            checked={item.id in selected}
                            onChange={(e) => toggleItem(item.id, e.target.checked)}
                          />
                          <span className="flex-1">
                            {item.name} <span className="text-muted-foreground">— {item.price.toLocaleString("ru-RU")} ₽/ед.</span>
                          </span>
                          {item.id in selected && (
                            <Input
                              type="number"
                              min="0"
                              step="1"
                              className="w-20 h-7"
                              value={selected[item.id]}
                              onChange={(e) => setQty(item.id, e.target.value)}
                            />
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                ))}

                <div>
                  <div className="flex items-center justify-between mb-2">
                    <p className="text-sm font-semibold">Свои строки</p>
                    <Button type="button" variant="outline" size="sm" onClick={addCustomLine}>
                      <Plus className="mr-1 h-3 w-3" />
                      Добавить строку
                    </Button>
                  </div>
                  {customLines.map((line, index) => (
                    <div key={index} className="grid grid-cols-12 gap-2 mb-2 items-center">
                      <Input
                        className="col-span-6 h-8"
                        placeholder="Наименование"
                        value={line.title}
                        onChange={(e) => updateCustomLine(index, { title: e.target.value })}
                      />
                      <Input
                        className="col-span-2 h-8"
                        placeholder="Ед."
                        value={line.unit}
                        onChange={(e) => updateCustomLine(index, { unit: e.target.value })}
                      />
                      <Input
                        className="col-span-2 h-8"
                        type="number"
                        placeholder="Цена"
                        value={line.price}
                        onChange={(e) => updateCustomLine(index, { price: e.target.value })}
                      />
                      <Input
                        className="col-span-1 h-8"
                        type="number"
                        placeholder="Кол-во"
                        value={line.qty}
                        onChange={(e) => updateCustomLine(index, { qty: e.target.value })}
                      />
                      <Button type="button" variant="ghost" size="sm" className="col-span-1" onClick={() => removeCustomLine(index)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <DialogFooter className="flex items-center justify-between sm:justify-between mt-4">
              <div className="text-sm font-semibold">Итого: {total.toLocaleString("ru-RU")} ₽</div>
              <Button onClick={handleSubmit} disabled={submitting}>
                {submitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                Создать
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {error && (
        <div className="mb-4 rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">{error}</div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : appendices.length === 0 && !error ? (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">Приложений пока нет</CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          {appendices.map((appendix) => (
            <Link key={appendix.id} href={`/${customerId}/contracts/${contractId}/appendices/${appendix.id}`}>
              <Card className="hover:shadow-md transition-shadow cursor-pointer">
                <CardHeader className="flex flex-row items-center gap-3 py-4">
                  <FileSpreadsheet className="h-5 w-5 text-primary" />
                  <div>
                    <CardTitle className="text-base">Приложение № {appendix.number}</CardTitle>
                    <p className="text-sm text-muted-foreground">
                      {appendix.date} • {appendix.total_amount.toLocaleString("ru-RU")} ₽
                    </p>
                  </div>
                </CardHeader>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

"use client"

import { Fragment, useEffect, useState } from "react"
import { Pencil, Plus, Trash2, Loader2 } from "lucide-react"
import { useRouter } from "next/navigation"

import { Service, servicesAPI, invoicesAPI, actsAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { TableCell, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

type EditableServiceLineProps = {
  docId: string
  docType: "invoice" | "act"
  service: Service
  index: number
}

type Mode = "edit" | "add"

function lineAmount(service: Service) {
  return service.amount ?? service.price * (service.qty || 1)
}

export default function EditableServiceLine({
  docId,
  docType,
  service,
  index,
}: EditableServiceLineProps) {
  const router = useRouter()
  const [services, setServices] = useState<Service[]>([])
  const [mode, setMode] = useState<Mode>("edit")
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [serviceId, setServiceId] = useState("")
  const [title, setTitle] = useState(service.name)
  const [unit, setUnit] = useState(service.unit || "шт")
  const [qty, setQty] = useState(String(service.qty || 1))
  const [price, setPrice] = useState(String(service.price))
  const [error, setError] = useState("")

  useEffect(() => {
    servicesAPI
      .getAll(1, 1000)
      .then((response) => setServices(response.data || []))
      .catch((err) => {
        console.error("Failed to load services:", err)
        setServices([])
      })
  }, [])

  const resetForEdit = () => {
    setMode("edit")
    setServiceId("")
    setTitle(service.name)
    setUnit(service.unit || "шт")
    setQty(String(service.qty || 1))
    setPrice(String(service.price))
    setError("")
    setOpen(true)
  }

  const resetForAdd = () => {
    setMode("add")
    setServiceId("")
    setTitle("")
    setUnit("шт")
    setQty("1")
    setPrice("")
    setError("")
    setOpen(true)
  }

  const payload = () => {
    const parsedQty = parseFloat(qty) || 1
    if (serviceId) {
      return { service_id: serviceId, qty: parsedQty }
    }
    return {
      title,
      unit,
      price: parseFloat(price),
      qty: parsedQty,
    }
  }

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const api = docType === "invoice" ? invoicesAPI : actsAPI
      if (mode === "edit") {
        await api.updateLine(docId, service.id, payload())
      } else {
        await api.addLine(docId, payload())
      }
      setOpen(false)
      router.refresh()
    } catch (err: unknown) {
      console.error("Failed to save line:", err)
      const message = err instanceof Error ? err.message : "Ошибка при сохранении услуги"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!confirm("Удалить услугу из документа?")) return
    setSubmitting(true)
    try {
      const api = docType === "invoice" ? invoicesAPI : actsAPI
      await api.deleteLine(docId, service.id)
      router.refresh()
    } catch (err: unknown) {
      console.error("Failed to delete line:", err)
      const message = err instanceof Error ? err.message : "Ошибка при удалении услуги"
      alert(message)
    } finally {
      setSubmitting(false)
    }
  }

  const convertToCost = (value: number | string) => {
    const amount = typeof value === "string" ? parseFloat(value.replace(",", ".")) : value
    const cleanPrice = amount.toFixed(2).replace(/[^\d.]/g, "")
    const parts = cleanPrice.split(".")
    const rubles = parts[0].length >= 3 ? `${parts[0].slice(0, -3)} ${parts[0].slice(-3)}` : parts[0]
    return `${rubles},${parts[1]}`
  }

  return (
    <Fragment>
      <TableRow className="group h-7 cursor-pointer print:border-black" onClick={resetForEdit}>
        <TableCell className="text-[10px] p-1 text-center print:border print:border-black">{index + 1}</TableCell>
        <TableCell className="text-[10px] p-1 print:border print:border-black">{service.name}</TableCell>
        <TableCell className="text-[10px] p-1 text-center print:border print:border-black">
          {service.unit || "шт"}
        </TableCell>
        <TableCell className="text-[10px] p-1 text-center print:border print:border-black">
          {service.qty || 1}
        </TableCell>
        <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">
          {convertToCost(service.price)}
        </TableCell>
        <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">
          {convertToCost(lineAmount(service))}
        </TableCell>
        <TableCell className="not-print w-[76px] p-1">
          <div className="flex justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={(event) => {
                event.stopPropagation()
                resetForEdit()
              }}
              title="Редактировать"
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive hover:text-destructive"
              disabled={submitting}
              onClick={(event) => {
                event.stopPropagation()
                handleDelete()
              }}
              title="Удалить"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        </TableCell>
      </TableRow>
      <TableRow className="not-print h-7">
        <TableCell colSpan={7} className="p-0">
          <button
            type="button"
            className="flex h-7 w-full items-center justify-center text-muted-foreground opacity-0 transition-opacity hover:bg-muted hover:opacity-100 focus:opacity-100"
            onClick={resetForAdd}
            title="Добавить услугу ниже"
          >
            <Plus className="h-4 w-4" />
          </button>
        </TableCell>
      </TableRow>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <form onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle>{mode === "edit" ? "Редактировать услугу" : "Добавить услугу"}</DialogTitle>
              <DialogDescription>Выберите готовую услугу или заполните поля вручную.</DialogDescription>
            </DialogHeader>
            {error && <Alert className="mt-4">{error}</Alert>}
            <div className="grid gap-4 py-4">
              <div className="space-y-2">
                <label htmlFor={`service-${service.id}`} className="text-sm font-medium">
                  Готовая услуга
                </label>
                <Select
                  id={`service-${service.id}`}
                  value={serviceId}
                  onChange={(event) => setServiceId(event.target.value)}
                >
                  <option value="">Не выбрано</option>
                  {services.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} • {item.price} ₽
                    </option>
                  ))}
                </Select>
              </div>

              {!serviceId && (
                <>
                  <div className="space-y-2">
                    <label htmlFor={`title-${service.id}`} className="text-sm font-medium">
                      Название
                    </label>
                    <Input
                      id={`title-${service.id}`}
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      required
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-2">
                      <label htmlFor={`unit-${service.id}`} className="text-sm font-medium">
                        Ед. изм.
                      </label>
                      <Input id={`unit-${service.id}`} value={unit} onChange={(event) => setUnit(event.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <label htmlFor={`price-${service.id}`} className="text-sm font-medium">
                        Цена
                      </label>
                      <Input
                        id={`price-${service.id}`}
                        type="number"
                        value={price}
                        onChange={(event) => setPrice(event.target.value)}
                        required
                      />
                    </div>
                  </div>
                </>
              )}

              <div className="space-y-2">
                <label htmlFor={`qty-${service.id}`} className="text-sm font-medium">
                  Количество
                </label>
                <Input id={`qty-${service.id}`} type="number" value={qty} onChange={(event) => setQty(event.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
                Отмена
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Сохранение...
                  </>
                ) : (
                  "Сохранить"
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Fragment>
  )
}

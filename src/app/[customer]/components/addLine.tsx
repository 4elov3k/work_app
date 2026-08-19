"use client"
import React, { useEffect, useState } from "react"
import { Loader2, Plus } from "lucide-react"
import { useRouter } from "next/navigation"

import { servicesAPI, invoicesAPI, actsAPI, Service } from "@/lib/api"
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

type AddLineProps = {
  docId: string
  docType: "invoice" | "act"
}

export default function AddLine({ docId, docType }: AddLineProps) {
  const router = useRouter()
  const [services, setServices] = useState<Service[]>([])
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [serviceId, setServiceId] = useState("")
  const [title, setTitle] = useState("")
  const [price, setPrice] = useState("")
  const [qty, setQty] = useState("1")
  const [unit, setUnit] = useState("шт")
  const [error, setError] = useState("")

  useEffect(() => {
    servicesAPI
      .getAll(1, 1000)
      .then((res) => setServices(res.data || []))
      .catch((err) => {
        console.error("Failed to load services:", err)
        setServices([])
      })
  }, [])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const line = serviceId
        ? { service_id: serviceId, qty: parseFloat(qty) || 1 }
        : { title, price: parseFloat(price), qty: parseFloat(qty) || 1, unit }

      if (docType === "invoice") {
        await invoicesAPI.addLine(docId, line)
      } else {
        await actsAPI.addLine(docId, line)
      }

      setServiceId("")
      setTitle("")
      setPrice("")
      setQty("1")
      setUnit("шт")
      setOpen(false)
      router.refresh()
    } catch (err: unknown) {
      console.error("Failed to add line:", err)
      const message = err instanceof Error ? err.message : "Ошибка при добавлении услуги"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <Plus className="mr-2 h-4 w-4" />
          Добавить услугу
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Добавить услугу</DialogTitle>
            <DialogDescription>Выберите услугу из справочника или укажите вручную.</DialogDescription>
          </DialogHeader>
          {error && <Alert>{error}</Alert>}
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="serviceSelect" className="text-sm font-medium">
                Справочник услуг
              </label>
              <Select
                id="serviceSelect"
                value={serviceId}
                onChange={(e) => setServiceId(e.target.value)}
              >
                <option value="">Не выбрано</option>
                {services.filter((service) => !service.archived).map((service) => (
                  <option key={service.id} value={service.id}>
                    {service.name} • {service.price} ₽
                  </option>
                ))}
              </Select>
            </div>
            {!serviceId && (
              <>
                <div className="space-y-2">
                  <label htmlFor="lineTitle" className="text-sm font-medium">
                    Название
                  </label>
                  <Input
                    id="lineTitle"
                    placeholder="Услуга"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="linePrice" className="text-sm font-medium">
                    Цена
                  </label>
                  <Input
                    id="linePrice"
                    type="number"
                    placeholder="5000"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="lineUnit" className="text-sm font-medium">
                    Ед. изм.
                  </label>
                  <Input id="lineUnit" value={unit} onChange={(e) => setUnit(e.target.value)} />
                </div>
              </>
            )}
            <div className="space-y-2">
              <label htmlFor="lineQty" className="text-sm font-medium">
                Количество
              </label>
              <Input id="lineQty" type="number" value={qty} onChange={(e) => setQty(e.target.value)} />
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
                  Добавление...
                </>
              ) : (
                "Добавить"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

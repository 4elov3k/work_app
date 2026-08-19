"use client"
import React, { useEffect, useState } from "react"
import { Loader2, Plus, Trash2, Wrench } from "lucide-react"

import { Service, servicesAPI } from "@/lib/api"
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
import { Alert } from "@/components/ui/alert"

export default function ServicesList() {
  const [services, setServices] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [isOpen, setIsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [name, setName] = useState("")
  const [price, setPrice] = useState("")
  const [error, setError] = useState("")
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Service | null>(null)

  const loadServices = () => {
    servicesAPI
      .getAll(1, 1000)
      .then((response) => setServices(response.data || []))
      .catch((err) => {
        console.error("Failed to load services:", err)
        setServices([])
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadServices()
  }, [])

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      await servicesAPI.create({ name, price: parseFloat(price) })
      setName("")
      setPrice("")
      setIsOpen(false)
      loadServices()
    } catch (err: unknown) {
      console.error("Failed to create service:", err)
      const message = err instanceof Error ? err.message : "Ошибка при создании услуги"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setSubmitting(true)
    setError("")
    try {
      await servicesAPI.delete(deleteTarget.id)
      setDeleteOpen(false)
      setDeleteTarget(null)
      loadServices()
    } catch (err: unknown) {
      console.error("Failed to delete service:", err)
      const message = err instanceof Error ? err.message : "Ошибка при удалении услуги"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div>
          <h3 className="text-lg font-semibold">Услуги</h3>
          <p className="text-sm text-muted-foreground">Всего: {services.length}</p>
        </div>

        <Dialog open={isOpen} onOpenChange={setIsOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Добавить услугу
            </Button>
          </DialogTrigger>
          <DialogContent>
            <form onSubmit={handleCreate}>
              <DialogHeader>
                <DialogTitle>Создать услугу</DialogTitle>
                <DialogDescription>Введите название и цену</DialogDescription>
              </DialogHeader>
              {error && <Alert>{error}</Alert>}
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <label htmlFor="serviceName" className="text-sm font-medium">
                    Название услуги
                  </label>
                  <Input
                    id="serviceName"
                    placeholder="Консультация"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
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
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    required
                  />
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
      ) : services.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <Wrench className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">Нет услуг</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {services.map((service) => (
            <Card key={service.id} className="hover:shadow-lg transition-shadow h-full">
              <CardHeader>
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <Wrench className="h-5 w-5 text-muted-foreground" />
                    <Badge variant="outline">{service.price} ₽</Badge>
                    {service.archived && <Badge variant="secondary">Архив</Badge>}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={(e) => {
                      e.preventDefault()
                      setDeleteTarget(service)
                      setDeleteOpen(true)
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
                <CardTitle className="text-xl">{service.name}</CardTitle>
                <CardDescription>ID: {service.id}</CardDescription>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Удалить услугу?</DialogTitle>
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

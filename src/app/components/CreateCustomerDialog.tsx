"use client"
import { useState } from "react"
import { Plus, Loader2, Building2, Search } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Alert } from "@/components/ui/alert"
import { Customer, customersAPI } from "@/lib/api"

interface CreateCustomerDialogProps {
  onCreated?: (customer: Customer) => void
}

export default function CreateCustomerDialog({ onCreated }: CreateCustomerDialogProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [lookupLoading, setLookupLoading] = useState(false)
  const [lookupMessage, setLookupMessage] = useState("")
  const [error, setError] = useState("")
  const [name, setName] = useState("")
  const [fullname, setFullname] = useState("")
  const [address, setAddress] = useState("")
  const [inn, setInn] = useState("")
  const [kpp, setKpp] = useState("")
  const [contactPerson, setContactPerson] = useState("")
  const [contactPosition, setContactPosition] = useState("")

  const resetForm = () => {
    setName("")
    setFullname("")
    setAddress("")
    setInn("")
    setKpp("")
    setContactPerson("")
    setContactPosition("")
    setLookupMessage("")
    setError("")
  }

  const handleLookup = async () => {
    const cleanInn = inn.replace(/\D/g, "")
    const cleanKpp = kpp.replace(/\D/g, "")
    if (cleanInn.length !== 10 && cleanInn.length !== 12) {
      setLookupMessage("ИНН должен содержать 10 или 12 цифр")
      return
    }
    if (cleanKpp && cleanKpp.length !== 9) {
      setLookupMessage("КПП должен содержать 9 цифр")
      return
    }

    setLookupLoading(true)
    setLookupMessage("")
    try {
      const response = await customersAPI.lookupByInn(cleanInn, cleanKpp)
      const found = response.data
      setName(found.name)
      setFullname(found.fullname)
      setAddress(found.address)
      setInn(found.inn)
      setKpp(found.kpp)
      setContactPerson(found.contact_person || "")
      setContactPosition(found.contact_position || "")
      setLookupMessage(found.status === "ACTIVE" ? "Реквизиты найдены" : `Найдено, статус: ${found.status || "не указан"}`)
    } catch (err) {
      console.error("Failed to lookup customer:", err)
      const message = err instanceof Error ? err.message : "Неизвестная ошибка"
      setLookupMessage(message)
    } finally {
      setLookupLoading(false)
    }
  }

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      const response = await customersAPI.create({
        name,
        fullname,
        address,
        inn: inn.replace(/\D/g, ""),
        kpp: kpp.replace(/\D/g, ""),
        contact_person: contactPerson,
        contact_position: contactPosition,
      })
      onCreated?.(response.data)

      setIsOpen(false)
      resetForm()
    } catch (err) {
      console.error('Failed to create customer:', err)
      setError(err instanceof Error ? err.message : 'Неизвестная ошибка')
    } finally {
      setSubmitting(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    setIsOpen(open)
    if (!open) {
      resetForm()
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="lg">
          <Plus className="mr-2 h-5 w-5" />
          Добавить контрагента
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[500px]">
        <form onSubmit={handleCreate}>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Building2 className="h-5 w-5" />
              Создать контрагента
            </DialogTitle>
            <DialogDescription>
              Введите данные нового контрагента. Все поля обязательны для заполнения.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            {error && <Alert>{error}</Alert>}
            <div className="space-y-2">
              <label htmlFor="name" className="text-sm font-medium">
                Краткое название <span className="text-destructive">*</span>
              </label>
              <Input
                id="name"
                placeholder="ООО Ромашка"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
              <p className="text-xs text-muted-foreground">
                Краткое название для отображения в списках
              </p>
            </div>
            
            <div className="space-y-2">
              <label htmlFor="fullname" className="text-sm font-medium">
                Полное наименование <span className="text-destructive">*</span>
              </label>
              <Input
                id="fullname"
                placeholder='Общество с ограниченной ответственностью "Ромашка"'
                value={fullname}
                onChange={(e) => setFullname(e.target.value)}
                required
              />
              <p className="text-xs text-muted-foreground">
                Полное юридическое название для документов
              </p>
            </div>

            <div className="space-y-2">
              <label htmlFor="inn" className="text-sm font-medium">
                ИНН <span className="text-destructive">*</span>
              </label>
              <div className="flex gap-2">
                <Input
                  id="inn"
                  placeholder="7701234567"
                  value={inn}
                  onChange={(e) => setInn(e.target.value.replace(/\D/g, ""))}
                  maxLength={12}
                  required
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleLookup}
                  disabled={lookupLoading || submitting}
                >
                  {lookupLoading ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Search className="h-4 w-4" />
                  )}
                  <span className="ml-2 hidden sm:inline">Проверить</span>
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                10 цифр для организаций, 12 для ИП
              </p>
              {lookupMessage && (
                <p className="text-xs text-muted-foreground">
                  {lookupMessage}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <label htmlFor="kpp" className="text-sm font-medium">
                КПП
              </label>
              <Input
                id="kpp"
                placeholder="770101001"
                value={kpp}
                onChange={(e) => setKpp(e.target.value.replace(/\D/g, ""))}
                maxLength={9}
              />
              <p className="text-xs text-muted-foreground">
                Обязательно для организаций, для ИП оставить пустым
              </p>
            </div>
            
            <div className="space-y-2">
              <label htmlFor="address" className="text-sm font-medium">
                Адрес <span className="text-destructive">*</span>
              </label>
              <Input
                id="address"
                placeholder="г. Москва, ул. Цветочная, д. 5"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                required
              />
              <p className="text-xs text-muted-foreground">
                Юридический адрес организации
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <label htmlFor="contactPerson" className="text-sm font-medium">
                  Контактное лицо
                </label>
                <Input
                  id="contactPerson"
                  placeholder="ФИО директора"
                  value={contactPerson}
                  onChange={(e) => setContactPerson(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <label htmlFor="contactPosition" className="text-sm font-medium">
                  Должность
                </label>
                <Input
                  id="contactPosition"
                  placeholder="Директор"
                  value={contactPosition}
                  onChange={(e) => setContactPosition(e.target.value)}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button 
              type="button" 
              variant="outline" 
              onClick={() => handleOpenChange(false)}
              disabled={submitting}
            >
              Отмена
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Создание...
                </>
              ) : (
                <>
                  <Plus className="mr-2 h-4 w-4" />
                  Создать
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

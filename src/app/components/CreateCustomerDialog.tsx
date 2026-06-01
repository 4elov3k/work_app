"use client"
import { useState } from "react"
import { Plus, Loader2, Building2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Customer, customersAPI } from "@/lib/api"

interface CreateCustomerDialogProps {
  onCreated?: (customer: Customer) => void
}

export default function CreateCustomerDialog({ onCreated }: CreateCustomerDialogProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [name, setName] = useState("")
  const [fullname, setFullname] = useState("")
  const [address, setAddress] = useState("")
  const [inn, setInn] = useState("")

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)

    try {
      const response = await customersAPI.create({
        name,
        fullname,
        address,
        inn
      })
      onCreated?.(response.data)

      setIsOpen(false)
      setName("")
      setFullname("")
      setAddress("")
      setInn("")
    } catch (err) {
      console.error('Failed to create customer:', err)
      const message = err instanceof Error ? err.message : 'Неизвестная ошибка'
      alert(`Ошибка при создании контрагента: ${message}`)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
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
              <Input
                id="inn"
                placeholder="7701234567"
                value={inn}
                onChange={(e) => setInn(e.target.value)}
                maxLength={12}
                required
              />
              <p className="text-xs text-muted-foreground">
                10 цифр для организаций, 12 для ИП
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
          </div>
          <DialogFooter>
            <Button 
              type="button" 
              variant="outline" 
              onClick={() => setIsOpen(false)} 
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

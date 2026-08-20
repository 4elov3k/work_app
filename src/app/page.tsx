"use client"
import { ChangeEvent, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { LayoutDashboard, Search, Building2, FileText, Phone } from "lucide-react";

import { customersAPI, Customer } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import CreateCustomerDialog from "./components/CreateCustomerDialog";
import DeleteCustomerButton from "./[customer]/components/deleteCustomerButton";


export default function Home() {
  const [items, setItems] = useState<Customer[]>([]);
  const [value, setValue] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Guards against the initial load (this effect) and the debounced search
  // effect below racing each other: whichever of the two fires last used to
  // win regardless of which one was actually requested most recently — a
  // slow initial getAll() resolving after a fast search response would
  // silently overwrite the search result with the unfiltered list. Every
  // fetch that starts bumps this shared counter and captures its value;
  // on resolution, a fetch only applies its result if no newer fetch
  // (from either effect) has started since.
  const latestRequestId = useRef(0)

  useEffect(() => {
    const requestId = ++latestRequestId.current
    customersAPI.getAll().then(response => {
      if (latestRequestId.current !== requestId) return
      setItems(response.data || [])
      setError('')
    }).catch(err => {
      console.error('Failed to load customers:', err)
      if (latestRequestId.current !== requestId) return
      setItems([])
      setError('Не удалось загрузить список контрагентов. Проверьте соединение с сервером и попробуйте ещё раз.')
    }).finally(() => {
      if (latestRequestId.current === requestId) setLoading(false)
    })
  }, [])

  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setValue(event.target.value)
  };

  useEffect(() => {
    const timer = setTimeout(() => {
      const requestId = ++latestRequestId.current
      if (value.trim() === '') {
        customersAPI.getAll().then(response => {
          if (latestRequestId.current !== requestId) return
          setItems(response.data || [])
          setError('')
        }).catch(err => {
          console.error('Failed to load customers:', err)
          if (latestRequestId.current !== requestId) return
          setItems([])
          setError('Не удалось загрузить список контрагентов. Проверьте соединение с сервером и попробуйте ещё раз.')
        })
      } else {
        customersAPI.search(value).then(response => {
          if (latestRequestId.current !== requestId) return
          setItems(response.data || [])
          setError('')
        }).catch(err => {
          console.error('Failed to search customers:', err)
          if (latestRequestId.current !== requestId) return
          setItems([])
          setError('Не удалось выполнить поиск контрагентов. Проверьте соединение с сервером и попробуйте ещё раз.')
        })
      }
    }, 300)

    return () => clearTimeout(timer)
  }, [value])

  const handleCustomerCreated = (customer: Customer): void => {
    setValue("")
    setItems((current) => [customer, ...current.filter((item) => item.id !== customer.id)])
  }

  return (
    <div className="container mx-auto py-8 px-4 max-w-7xl">
      <div className="mb-8 flex items-start justify-between">
        <div>
          <h1 className="text-4xl font-bold tracking-tight mb-2">Контрагенты</h1>
          <p className="text-muted-foreground">Управление клиентами и контрагентами</p>
        </div>
        <div className="flex gap-2">
          <Link href="/redmine">
            <Button variant="outline">
              <LayoutDashboard className="mr-2 h-4 w-4" />
              Проекты Redmine
            </Button>
          </Link>
          <Link href="/invoices">
            <Button variant="outline">
              <FileText className="mr-2 h-4 w-4" />
              Все документы
            </Button>
          </Link>
          <Link href="/zvonari">
            <Button variant="outline">
              <Phone className="mr-2 h-4 w-4" />
              Звонари
            </Button>
          </Link>
          <CreateCustomerDialog onCreated={handleCustomerCreated} />
        </div>
      </div>

      <div className="mb-6 relative">
        <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
        <Input
          type="text"
          placeholder="Поиск контрагентов по названию или ИНН..."
          aria-label="Поиск контрагентов по названию или ИНН"
          value={value}
          onChange={handleSearchChange}
          className="pl-10"
        />
      </div>

      {error && <Alert className="mb-6">{error}</Alert>}

      {loading ? (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="animate-pulse">
              <CardHeader>
                <div className="h-4 bg-muted rounded w-3/4 mb-2"></div>
                <div className="h-3 bg-muted rounded w-1/2"></div>
              </CardHeader>
            </Card>
          ))}
        </div>
      ) : items.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <Building2 className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">
              {value ? 'Контрагенты не найдены' : 'Нет контрагентов'}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {items.map((item) => (
            <Link key={item.id} href={`/${item.id}`}>
              <Card className="h-full hover:shadow-lg transition-shadow cursor-pointer">
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <Building2 className="h-5 w-5 text-muted-foreground" />
                    <div className="flex items-center gap-1">
                      <Badge variant="secondary">
                        <FileText className="h-3 w-3 mr-1" />
                        Активен
                      </Badge>
                      <DeleteCustomerButton
                        customerId={item.id}
                        customerName={item.name}
                        onDeleted={() => setItems((current) => current.filter((c) => c.id !== item.id))}
                      />
                    </div>
                  </div>
                  <CardTitle className="text-xl">{item.name}</CardTitle>
                  <CardDescription className="line-clamp-1">
                    {item.fullname}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center text-sm text-muted-foreground">
                    <span className="font-medium mr-1">ИНН:</span>
                    <span>{item.inn}</span>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

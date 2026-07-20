"use client"
import { useState, useEffect } from "react"
import { Invoice, Act, invoicesAPI, actsAPI, customersAPI, Customer } from "@/lib/api"
import Link from "next/link"
import { ArrowLeft, FileText, FileCheck, Calendar, User, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Input } from "@/components/ui/input"

export default function AllInvoicesPage() {
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [acts, setActs] = useState<Act[]>([])
  const [customers, setCustomers] = useState<Record<string, Customer>>({})
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState("")

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    try {
      // Загружаем всех клиентов
      const customersResponse = await customersAPI.getAll(1, 1000)
      const customersMap: Record<string, Customer> = {}
      customersResponse.data.forEach(customer => {
        customersMap[customer.id] = customer
      })
      setCustomers(customersMap)

      // Загружаем все счета
      const invoicesResponse = await invoicesAPI.getByCustomer("", "", "all", 1, 1000)
      setInvoices(invoicesResponse.data || [])

      // Загружаем все акты
      const actsResponse = await actsAPI.getByCustomer("", "", "all", 1, 1000)
      setActs(actsResponse.data || [])
    } catch (err) {
      console.error('Failed to load data:', err)
    } finally {
      setLoading(false)
    }
  }

  const documentTime = (date: string) => {
    const [day, month, year] = date.split(".").map(Number)
    return new Date(year, month - 1, day).getTime()
  }

  const sortDocumentsByDate = (documents: Array<Invoice | Act>) => {
    return [...documents].sort((left, right) => {
      const dateDiff = documentTime(right.date) - documentTime(left.date)
      if (dateDiff !== 0) return dateDiff
      return right.number.localeCompare(left.number, "ru", { numeric: true })
    })
  }

  const filterDocuments = (documents: Array<Invoice | Act>) => {
    const filtered = searchQuery ? documents.filter(doc => {
      const customer = customers[doc.customer_id]
      const customerName = customer?.name?.toLowerCase() || ""
      const number = doc.number.toLowerCase()
      const date = doc.date.toLowerCase()
      const query = searchQuery.toLowerCase()
      
      return customerName.includes(query) || 
             number.includes(query) || 
             date.includes(query)
    }) : documents

    return sortDocumentsByDate(filtered)
  }

  const renderDocumentCard = (doc: Invoice | Act, type: "invoice" | "act") => {
    const customer = customers[doc.customer_id]
    const icon = type === "invoice" ? FileText : FileCheck
    const Icon = icon

    return (
      <Link key={doc.id} href={type === "act" ? `/${doc.customer_id}/acts/${doc.id}` : `/${doc.customer_id}/${doc.id}`}>
        <Card className="hover:shadow-lg transition-shadow cursor-pointer h-full">
          <CardHeader>
            <div className="flex items-start justify-between">
              <Icon className="h-5 w-5 text-muted-foreground" />
              <Badge variant={type === "invoice" ? "outline" : "secondary"}>
                {type === "invoice" ? "Счет" : "Акт"}
              </Badge>
            </div>
            <CardTitle className="text-xl">№ {doc.number}</CardTitle>
            <CardDescription className="space-y-1">
              <div className="flex items-center gap-1">
                <Calendar className="h-3 w-3" />
                {doc.date}
              </div>
              {customer && (
                <div className="flex items-center gap-1">
                  <User className="h-3 w-3" />
                  {customer.name}
                </div>
              )}
              {doc.contract_number && (
                <div className="text-xs text-muted-foreground">
                  Договор: {doc.contract_number}
                </div>
              )}
            </CardDescription>
          </CardHeader>
        </Card>
      </Link>
    )
  }

  return (
    <div className="container mx-auto py-8 px-4 max-w-7xl">
      <div className="mb-6">
        <div className="flex gap-4 items-center mb-6">
          <Link href="/">
            <Button variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Главная
            </Button>
          </Link>
          <h1 className="text-3xl font-bold">Все документы</h1>
        </div>

        {/* Поиск */}
        <div className="max-w-md">
          <Input
            placeholder="Поиск по номеру, дате или клиенту..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
      </div>

      <Tabs defaultValue="invoices" className="w-full">
        <TabsList className="grid w-full max-w-md grid-cols-2">
          <TabsTrigger value="invoices" className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Счета ({invoices.length})
          </TabsTrigger>
          <TabsTrigger value="acts" className="flex items-center gap-2">
            <FileCheck className="h-4 w-4" />
            Акты ({acts.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value="invoices" className="mt-6">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : filterDocuments(invoices).length === 0 ? (
            <Card>
              <CardContent className="py-10 text-center">
                <FileText className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
                <p className="text-muted-foreground">
                  {searchQuery ? "Счета не найдены" : "Нет счетов"}
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {filterDocuments(invoices).map(invoice => renderDocumentCard(invoice, "invoice"))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="acts" className="mt-6">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : filterDocuments(acts).length === 0 ? (
            <Card>
              <CardContent className="py-10 text-center">
                <FileCheck className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
                <p className="text-muted-foreground">
                  {searchQuery ? "Акты не найдены" : "Нет актов"}
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {filterDocuments(acts).map(act => renderDocumentCard(act, "act"))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}

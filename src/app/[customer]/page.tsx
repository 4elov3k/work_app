import { ArrowLeft, Building2, MapPin, Hash, AlertTriangle } from "lucide-react"
import Link from "next/link"
import { notFound } from "next/navigation"
import Lists from "./components/lists"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { customersAPI, ApiError } from "@/lib/api"
import RedmineProjectLinkPanel from "./components/redmineProjectLink"
import DeleteCustomerButton from "./components/deleteCustomerButton"

export interface IData {
  collectionId?: string
  collectionName?: string
  created?: string
  id?: string
  name?: string
  adress?: string
  inn?: string
  updated?: string
}

export interface Iitem {
	collectionId?: string
	collectionName?: string
	id?: string
	customer?: string
	services?: string[]
	number?: string
	date?: string
  name?: string
  price?: string
	created?: string
	updated?: string
}

export default async function Page({
    params,
  }: {
    params: Promise<{ customer: string }>
  }) {
    const slug = (await params).customer

    // Получаем данные контрагента
    let customer
    let loadError = ''
    try {
      const response = await customersAPI.getById(slug)
      customer = response.data
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        notFound()
      }
      console.error('Failed to load customer:', error)
      loadError = 'Не удалось загрузить контрагента. Проверьте соединение с сервером и попробуйте обновить страницу.'
    }

    return (
      <div className="container mx-auto py-8 px-4 max-w-7xl">
        <div className="mb-6">
          <Link href="/">
            <Button variant="ghost" className="mb-4">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Назад к списку
            </Button>
          </Link>
          
          {loadError && (
            <div className="mb-4 flex items-center gap-2 rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              {loadError}
            </div>
          )}

          {customer && (
            <Card>
              <CardHeader>
                <div className="flex items-start gap-4">
                  <div className="p-3 rounded-lg bg-primary/10">
                    <Building2 className="h-8 w-8 text-primary" />
                  </div>
                  <div className="flex-1">
                    <CardTitle className="text-2xl mb-1">{customer.name}</CardTitle>
                    <CardDescription className="text-base">{customer.fullname}</CardDescription>
                  </div>
                  <DeleteCustomerButton customerId={slug} customerName={customer.name} />
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex items-center gap-2 text-sm">
                    <Hash className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium">ИНН:</span>
                    <span>{customer.inn}</span>
                  </div>
                  {customer.address && (
                    <div className="flex items-center gap-2 text-sm">
                      <MapPin className="h-4 w-4 text-muted-foreground" />
                      <span className="font-medium">Адрес:</span>
                      <span className="line-clamp-1">{customer.address}</span>
                    </div>
                  )}
                </div>
                <RedmineProjectLinkPanel customerId={slug} />
              </CardContent>
            </Card>
          )}
        </div>

        {customer && (
          <>
            <Separator className="my-6" />
            <Lists slug={slug}/>
          </>
        )}
      </div>
    )
  }

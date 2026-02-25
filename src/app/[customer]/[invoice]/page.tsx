import FormCus from "@/app/components/components/form"
import Link from "next/link"
import Print from "./components/print"
import Duplicate from "./components/duplicate"
import DownloadPdf from "./components/downloadPdf"
import { ArrowLeft, Home } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { invoicesAPI } from "@/lib/api.server"
import AddLine from "../components/addLine"
import EditDocument from "../components/editDocument"
import CreateActFromInvoice from "./components/createActFromInvoice"

export default async function Page({
    params
  }: {
    params: Promise<{ invoice: string; customer: string }>
  }) {
    const { invoice: invoiceId, customer: customerId } = await params
    
    // Получаем данные для формирования имени файла
    const invoiceData = await invoicesAPI.getById(invoiceId)
    const invoice = invoiceData.data
    const fileName = `Счет_${invoice.number}_${invoice.date.replace(/\./g, '-')}`
    
    return (
        <div className="container mx-auto py-8 px-4 max-w-5xl">
          <div className="not-print mb-6">
              <div className="flex gap-4 items-center mb-6">
                  <Link href={`/${customerId}`}>
                    <Button variant="outline">
                      <ArrowLeft className="mr-2 h-4 w-4" />
                      Назад
                    </Button>
                  </Link>
                  <Link href="/">
                    <Button variant="outline">
                      <Home className="mr-2 h-4 w-4" />
                      Главная
                    </Button>
                  </Link>
                  {invoice.archived && <Badge variant="secondary">В архиве</Badge>}
                  <div className="flex-1"></div>
                  <EditDocument
                    docType="invoice"
                    docId={invoiceId}
                    number={invoice.number}
                    date={invoice.date}
                    status={invoice.status}
                    archived={invoice.archived}
                  />
                  {!invoice.archived && <AddLine docId={invoiceId} docType="invoice" />}
                  {!invoice.archived && <CreateActFromInvoice invoiceId={invoiceId} customerId={customerId} />}
                  <DownloadPdf fileName={fileName} />
                  <Duplicate invoiceId={invoiceId} customerId={customerId} />
                  <Print/>
              </div>
              <Separator />
          </div>
                
          <FormCus id={invoiceId} type="invoice"/>
        </div>
    )
  }

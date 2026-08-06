import Link from "next/link"
import { ArrowLeft, Home, FileCheck, FileText, FileSpreadsheet, Folder } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import DocumentList from "../../components/documentList"
import AppendixList from "./components/appendixList"
import DeleteContractButton from "./components/deleteContractButton"
import { contractsAPI, customersAPI } from "@/lib/api.server"

export default async function ContractPage({
  params,
}: {
  params: Promise<{ customer: string; contract: string }>
}) {
  const { customer: customerId, contract: contractId } = await params

  const contractResponse = await contractsAPI.getById(contractId)
  const contract = contractResponse.data

  const customerResponse = await customersAPI.getById(customerId)
  const customer = customerResponse.data

  return (
    <div className="container mx-auto py-8 px-4 max-w-7xl">
      <div className="mb-6">
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
        </div>

        <Card>
          <CardHeader>
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-lg bg-primary/10">
                <Folder className="h-8 w-8 text-primary" />
              </div>
              <div className="flex-1">
                <CardTitle className="text-2xl mb-1">Договор № {contract.number}</CardTitle>
                <CardDescription className="text-base">
                  {customer.name} • {contract.topic}
                </CardDescription>
              </div>
              <DeleteContractButton
                customerId={customerId}
                contractId={contractId}
                contractNumber={contract.number}
              />
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="text-sm">
                <span className="font-medium">Статус:</span>{" "}
                {contract.status === "archived" ? "Архив" : "Активен"}
              </div>
              <div className="text-sm">
                <span className="font-medium">Дата:</span> {contract.start_date || "Без даты"}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Separator className="my-6" />

      <Tabs defaultValue="acts" className="w-full">
        <TabsList className="grid w-full max-w-lg grid-cols-3">
          <TabsTrigger value="acts" className="flex items-center gap-2">
            <FileCheck className="h-4 w-4" />
            Акты
          </TabsTrigger>
          <TabsTrigger value="invoices" className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Счета
          </TabsTrigger>
          <TabsTrigger value="appendices" className="flex items-center gap-2">
            <FileSpreadsheet className="h-4 w-4" />
            Приложения
          </TabsTrigger>
        </TabsList>

        <TabsContent value="acts" className="mt-6">
          <DocumentList slug={customerId} documentType="act" fixedContractId={contractId} />
        </TabsContent>

        <TabsContent value="invoices" className="mt-6">
          <DocumentList slug={customerId} documentType="invoice" fixedContractId={contractId} />
        </TabsContent>

        <TabsContent value="appendices" className="mt-6">
          <AppendixList customerId={customerId} contractId={contractId} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

import Link from "next/link"
import { notFound } from "next/navigation"
import { ArrowLeft, Home } from "lucide-react"

import { Button } from "@/components/ui/button"
import Print from "../../../../[invoice]/components/print"
import ContractAppendixTemplate from "@/app/components/templates/ContractAppendixTemplate"
import { contractsAPI, customersAPI, contractAppendicesAPI, organizationAPI, ApiError } from "@/lib/api.server"

export default async function ContractAppendixPage({
  params,
}: {
  params: Promise<{ customer: string; contract: string; appendix: string }>
}) {
  const { customer: customerId, contract: contractId, appendix: appendixId } = await params

  let appendixData
  try {
    appendixData = await contractAppendicesAPI.getById(appendixId)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound()
    }
    throw err
  }
  const appendix = appendixData.data

  const [contractResponse, customerResponse, organizationResponse] = await Promise.all([
    contractsAPI.getById(contractId),
    customersAPI.getById(customerId),
    organizationAPI.get(),
  ])

  return (
    <div className="container mx-auto py-8 px-4 max-w-5xl">
      <div className="not-print mb-6">
        <div className="flex flex-wrap gap-4 items-center mb-6">
          <Link href={`/${customerId}/contracts/${contractId}`}>
            <Button variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Назад к договору
            </Button>
          </Link>
          <Link href="/">
            <Button variant="outline">
              <Home className="mr-2 h-4 w-4" />
              Главная
            </Button>
          </Link>
          <Print />
        </div>
      </div>

      <ContractAppendixTemplate
        appendix={appendix}
        contract={contractResponse.data}
        customer={customerResponse.data}
        organization={organizationResponse.data}
      />
    </div>
  )
}

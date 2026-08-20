import './form.css'
import { notFound } from 'next/navigation'
import { customersAPI, invoicesAPI, actsAPI, organizationAPI, ApiError } from '@/lib/api.server'
import InvoiceTemplate from '../templates/InvoiceTemplate'
import CertificateTemplate from '../templates/CertificateTemplate'

export default async function FormCus ({id, type}:{id: string, type: "invoice" | "act"}) {

    // FormCus does its own fetch of the document and its customer,
    // independent of whatever the parent page already loaded — a 404 here
    // (e.g. a customer_id that no longer resolves) must be handled the same
    // way the parent pages handle their own not-found case, or it bypasses
    // notFound() entirely and hits the generic root error.tsx instead.
    let invoiceData, customer
    try {
      const documentWithServicesResponse = type === "act"
        ? await actsAPI.getWithServices(id)
        : await invoicesAPI.getWithServices(id)
      invoiceData = documentWithServicesResponse.data

      // Получаем данные контрагента
      const customerResponse = await customersAPI.getById(invoiceData.customer_id)
      customer = customerResponse.data
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        notFound()
      }
      throw err
    }

    // Реквизиты продавца (наша организация) — единый источник для печатных форм
    const organizationResponse = await organizationAPI.get()
    const organization = organizationResponse.data

    // Выбираем шаблон в зависимости от типа документа
    if (type === "act") {
      return <CertificateTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
    }

    return <InvoiceTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
}

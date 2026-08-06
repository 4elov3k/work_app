import './form.css'
import { customersAPI, invoicesAPI, actsAPI, organizationAPI } from '@/lib/api.server'
import InvoiceTemplate from '../templates/InvoiceTemplate'
import CertificateTemplate from '../templates/CertificateTemplate'

export default async function FormCus ({id, type}:{id: string, type: "invoice" | "act"}) {

    const documentWithServicesResponse = type === "act"
      ? await actsAPI.getWithServices(id)
      : await invoicesAPI.getWithServices(id)
    const invoiceData = documentWithServicesResponse.data

    // Получаем данные контрагента
    const customerResponse = await customersAPI.getById(invoiceData.customer_id)
    const customer = customerResponse.data

    // Реквизиты продавца (наша организация) — единый источник для печатных форм
    const organizationResponse = await organizationAPI.get()
    const organization = organizationResponse.data

    // Выбираем шаблон в зависимости от типа документа
    if (type === "act") {
      return <CertificateTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
    }

    return <InvoiceTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
}

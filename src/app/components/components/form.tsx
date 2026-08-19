import './form.css'
import { customersAPI, invoicesAPI, actsAPI, organizationAPI } from '@/lib/api.server'
import InvoiceTemplate from '../templates/InvoiceTemplate'
import CertificateTemplate from '../templates/CertificateTemplate'

export default async function FormCus ({id, type}:{id: string, type: "invoice" | "act"}) {

    // organization не зависит от документа/контрагента — запускаем сразу,
    // параллельно с загрузкой документа, вместо ожидания в конце цепочки.
    const documentPromise = type === "act"
      ? actsAPI.getWithServices(id)
      : invoicesAPI.getWithServices(id)
    const organizationPromise = organizationAPI.get()

    const documentWithServicesResponse = await documentPromise
    const invoiceData = documentWithServicesResponse.data

    // Получаем данные контрагента (customer_id известен только после документа)
    const customerResponse = await customersAPI.getById(invoiceData.customer_id)
    const customer = customerResponse.data

    // Реквизиты продавца (наша организация) — единый источник для печатных форм
    const organizationResponse = await organizationPromise
    const organization = organizationResponse.data

    // Выбираем шаблон в зависимости от типа документа
    if (type === "act") {
      return <CertificateTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
    }

    return <InvoiceTemplate invoice={invoiceData} customer={customer} docId={id} organization={organization} />
}

import './form.css'
import { customersAPI, invoicesAPI, actsAPI } from '@/lib/api.server'
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
    
    // Выбираем шаблон в зависимости от типа документа
    if (type === "act") {
      return <CertificateTemplate invoice={invoiceData} customer={customer} />
    }
    
    return <InvoiceTemplate invoice={invoiceData} customer={customer} />
}

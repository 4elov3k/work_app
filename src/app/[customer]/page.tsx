import axios from "axios"
import Link from "next/link"
import Back from "../components/components/routeBack"

export interface IData {
  collectionId: string
  collectionName: string
  created: string
  id: string
  name: string
  adress: string
  inn: string
  updated: string
}
export interface Iitem {
	collectionId: string
	collectionName: string
	id: string
	customer: string
	services: string[]
	number: string
	date: string
	created: string
	updated: string
  }


export default async function Page({
    params,
  }: {
    params: Promise<{ customer: string }>
  }) {
    const slug = (await params).customer
    const data: IData = await axios.get(`http://127.0.0.1:8090/api/collections/customers/records/${slug}`).then(resp => resp.data)
    const items: Iitem[] = await axios.get(`http://127.0.0.1:8090/api/collections/invoices/records?filter=(customer='${slug}')`).then(resp => resp.data.items)

    
    return (
      <div className="wrapper">
		<Back/>
		
		Инфо:
        <div> {data.name}</div>
        <div> {data.adress}</div>
        <div> {data.inn}</div>
		<ul>
			{items.map((item) => (
				<li key={item.id} className="custumer">
				<div>
					<strong><Link href={`/${slug}`+'/'+ `${item.id ?? '/'}`}>{item.number}</Link></strong>
				</div>
				<div>
					{item.date}
				</div>
			  </li>
			))}

			<li className="custumer text-center cursor-pointer">Добавить счет</li>
				
		</ul>
      </div>
      

    )
  }


  
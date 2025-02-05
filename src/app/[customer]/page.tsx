
import axios from "axios"
import Back from "../components/components/routeBack"
import InvoicesList from "./components/invoicesList"

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

	
    
	
	
    
    return (
      <div className="wrapper">
		<Back/>
		<InvoicesList slug={slug}/>
		
      </div>
      

    )
  }


  
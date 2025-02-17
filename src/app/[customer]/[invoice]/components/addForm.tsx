"use client"

import axios from "axios"
import { useState } from "react"


export default function AddForm ({id}:{id:string}) {
    


    const [number, setNumber] =useState<string>("")
    const [date, setDate] =useState<string>("")
    const [serviceName, setServiceName] =useState<string>("")
    const [servicePrice, setServicePrice] =useState<string>("")
    const [serviceDefault, setServiceDefault] =useState<boolean>(true)
    let test =[]
    const invoicesList = axios.get(`http://127.0.0.1:8090/api/collections/invoices/records/${id}`).then(data => {
        return axios.get(`http://127.0.0.1:8090/api/collections/invoices/records?filter=(customer="${data.data.customer}")`).then((data) => {data.data.items.map(item => test.push(item.services)); console.log(test)}  )
    })
    
    
    const handleClickAdd = () => {

    }
    return(
        <div className="flex border border-[var(--foreground)] p-4 m-8 gap-4 flex-col w-1/2 mx-auto">
        
            <span>{id}</span>
            {}
            <input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setNumber(event.target.value)} } value={number} placeholder="Номер счета" className="border  border-gray-50 bg-[var(--foreground)]"/>
            <input type="date" pattern="dd-mm-yyyy" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setDate(event.target.value)} } value={date} placeholder="Дата счета" className="border  border-gray-50 bg-[var(--foreground)]"/>
            <input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServiceName(event.target.value)} } value={serviceName} placeholder="Название услуги" className="border  border-gray-50 bg-[var(--foreground)]"/>
            <input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServicePrice(event.target.value)} } value={servicePrice} placeholder="Цена" className="border  border-gray-50 bg-[var(--foreground)]"/>
            <input type="checkbox" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServiceDefault(event.target.checked)}}/>
            <button className="text-center btn" onClick={handleClickAdd}>Изменить</button>	

      </div>
    )
}
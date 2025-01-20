"use client"
import { ChangeEvent, useEffect, useState } from "react";
import Link from "next/link";

import axios from "axios";


export interface Root {
  items: Item[]
  page: number
  perPage: number
  totalItems: number
  totalPages: number
}

export interface Item {
  collectionId: string
  collectionName: string
  created: string
  id: string
  name: string
  updated: string
}



export default function Home() {
const [items, setItems] = useState<Item[]>([]);
const [value, setValue] = useState('')

useEffect(()=>{
  axios.get("http://127.0.0.1:8090/api/collections/customers/records").then(resp => resp.data).then(data => {
    if(!data.items) {
      setItems([])
      return
    }
    setItems(data.items)
  })
}, [])

const handleSearchChange = (event: ChangeEvent<HTMLInputElement>): void => {
  setValue(event.target.value)
  
};
useEffect(() => {
  axios.get(`http://127.0.0.1:8090/api/collections/customers/records?filter=(name~'${value}')`)
      .then(resp => setItems(resp.data.items))
  
}, [value])
  

  return (
    <div className="wrapper">

        <h1>Список контрагентов</h1>
        <input type="text"
        
        onChange={handleSearchChange} 
        placeholder="Поиск контрагентов..." className="border mt-24 ml-6 border-gray-50 bg-[var(--background)]"/>
        {
          <ul className="custumer-list">
            {items.map( (item) => (

                
                  <li key={item.id} className="custumer">
                    <div>
                        <strong><Link href={`/${item.id ?? '/'}`}>{item.name}</Link></strong>
                    </div>
                    <div>
                        {item.name} ИНН: {item.id}
                    </div>
                  </li>
                
              ) )}
          </ul>
        }
       
        
    </div>
  );
}



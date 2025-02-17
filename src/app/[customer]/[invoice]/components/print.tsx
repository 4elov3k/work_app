"use client"


export default function Print () {
    


    const print = () => window.print();
    return(
        <div  className="w-24 btn my-8 p-4 border border-[var(--foreground)] cursor-pointer">
        <button onClick={print}>Печать</button>
        </div>
    )
}
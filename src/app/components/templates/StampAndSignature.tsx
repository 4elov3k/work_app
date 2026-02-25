export function Stamp() {
  return (
    <div data-stamp className="hidden print:block absolute" style={{ width: '120px', height: '120px' }}>
      <svg width="120" height="120" viewBox="0 0 120 120" xmlns="http://www.w3.org/2000/svg">
        {/* Внешний круг */}
        <circle cx="60" cy="60" r="58" fill="none" stroke="#0088cc" strokeWidth="1.5" opacity="0.7"/>
        <circle cx="60" cy="60" r="54" fill="none" stroke="#0088cc" strokeWidth="1" opacity="0.7"/>
        
        {/* Текст по кругу - верхняя часть */}
        <path id="circlePath1" d="M 20,60 A 40,40 0 0,1 100,60" fill="none"/>
        <text fill="#0088cc" fontSize="7" fontWeight="500" opacity="0.7">
          <textPath href="#circlePath1" startOffset="50%" textAnchor="middle">
            ИП МЫЛЬНИКОВА
          </textPath>
        </text>
        
        {/* Текст по кругу - нижняя часть */}
        <path id="circlePath2" d="M 100,60 A 40,40 0 0,1 20,60" fill="none"/>
        <text fill="#0088cc" fontSize="7" fontWeight="500" opacity="0.7">
          <textPath href="#circlePath2" startOffset="50%" textAnchor="middle">
            ЛЮБОВЬ ВАЛЕРЬЕВНА
          </textPath>
        </text>
        
        {/* Центральный текст */}
        <text x="60" y="55" textAnchor="middle" fill="#0088cc" fontSize="8" fontWeight="600" opacity="0.7">
          ИНН
        </text>
        <text x="60" y="68" textAnchor="middle" fill="#0088cc" fontSize="7" fontWeight="500" opacity="0.7">
          526220116209
        </text>
      </svg>
    </div>
  )
}

export function Signature() {
  return (
    <div data-stamp className="hidden print:block" style={{ 
      fontFamily: 'cursive', 
      fontSize: '16px', 
      color: '#000080',
      fontWeight: 500,
      fontStyle: 'italic',
      transform: 'rotate(-5deg)'
    }}>
      Л.В. Мыльникова
    </div>
  )
}

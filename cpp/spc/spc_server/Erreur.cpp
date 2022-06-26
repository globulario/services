#include "Erreur.h"
#if defined(WIN32) && defined(_DEBUG)
#define DEBUG_NEW new( _NORMAL_BLOCK, __FILE__, __LINE__ )
#define new DEBUG_NEW
#endif

#include <QJsonArray>

//Constructeur et Destructeur
Erreur::Erreur()
{}

Erreur::Erreur(int noErreur)
{
    noErreurs.push_back(noErreur);
}

Erreur::~Erreur()
{}

string Erreur::getDescriptionErreur(int noErreur)
{
    string erreur = "";
    if(noErreur == 1)
    {
        erreur = "1 point plus grand que k �carts type � partir de la ligne centrale";
    }
    else if(noErreur == 2)
    {
        erreur = "K points cons�cutifs, du m�me c�t� de la ligne centrale ";
    }
    else if(noErreur == 3)
    {
        erreur = "K points cons�cutifs, tous croissants ou tous d�croissant";
    }
    else if(noErreur == 4)
    {
        erreur = "K points cons�cutifs, croissants et d�croissant en alternance";
    }
    else if(noErreur == 5)
    {
        erreur = "k sur K+1 points > 2 �carts types � partir de la ligne centrale(du m�me c�t�)";
    }
    else if(noErreur == 6)
    {
        erreur = "K sur K+1 > 1 �cart type � partir de la ligne centrale (du m�me c�t�)";
    }
    else if(noErreur == 7)
    {
        erreur = "K points cons�cutifs, dans 1 �cart type de la ligne centrale (du m�me c�t�)";
    }
    else if(noErreur == 8)
    {
        erreur = "K points cons�cutifs > 1 �cart type � partir de la ligne centrale (des deux c�t�s)";
    }
    return erreur;
}

// Conversion to and from json values.
void Erreur::read(const QJsonObject &json){
    QJsonArray numbers = json["numbers"].toArray();
    for(size_t i =0; i < numbers.size(); i++){
        this->noErreurs.push_back(numbers[i].toDouble());
    }
}

void Erreur::write(QJsonObject &json) const{
    QJsonArray numbers;

    for(size_t i=0; i<this->noErreurs.size(); i++){
        double errorNumber = this->noErreurs[i];
        numbers.append(errorNumber);
    }
    json["numbers"] = numbers;
}

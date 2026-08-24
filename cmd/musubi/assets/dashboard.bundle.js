(()=>{var Li={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2};var Tf=0,Fh=1,Cf=2;var ku=1,Rf=2,Un=3,ei=0,ze=1,Nn=2,Sn=0,fs=1,ni=2,Bh=3,zh=4,Pf=5,bi=100,If=101,Lf=102,Df=103,Uf=104,Nf=200,Of=201,Ff=202,Bf=203,xc=204,vc=205,zf=206,kf=207,Hf=208,Vf=209,Gf=210,Wf=211,Xf=212,qf=213,Yf=214,_c=0,yc=1,Mc=2,xs=3,bc=4,Sc=5,wc=6,Ac=7,Hu=0,Zf=1,jf=2,ti=0,Kf=1,Jf=2,Qf=3,$f=4,tp=5,ep=6,np=7;var Vu=300,vs=301,_s=302,Ec=303,Tc=304,Ho=306,Cc=1e3,Ai=1001,Rc=1002,Ee=1003,ip=1004;var Br=1005;var je=1006,Ha=1007;var Ei=1008;var Fn=1009,Gu=1010,Wu=1011,ar=1012,Ul=1013,Ti=1014,bn=1015,Ve=1016,Nl=1017,Ol=1018,ys=1020,Xu=35902,qu=1021,Yu=1022,fn=1023,Zu=1024,ju=1025,ps=1026,Ms=1027,Fl=1028,Bl=1029,Ku=1030,zl=1031;var kl=1033,ao=33776,co=33777,lo=33778,ho=33779,Pc=35840,Ic=35841,Lc=35842,Dc=35843,Uc=36196,Nc=37492,Oc=37496,Fc=37808,Bc=37809,zc=37810,kc=37811,Hc=37812,Vc=37813,Gc=37814,Wc=37815,Xc=37816,qc=37817,Yc=37818,Zc=37819,jc=37820,Kc=37821,uo=36492,Jc=36494,Qc=36495,Ju=36283,$c=36284,tl=36285,el=36286;var po=2300,nl=2301,Va=2302,kh=2400,Hh=2401,Vh=2402;var sp=3200,rp=3201;var Qu=0,op=1,Qn="",yn="srgb",si="srgb-linear",Hl="display-p3",Vo="display-p3-linear",mo="linear",ie="srgb",go="rec709",xo="p3";var Ki=7680;var Gh=519,ap=512,cp=513,lp=514,$u=515,hp=516,up=517,dp=518,fp=519,Wh=35044;var Xh="300 es",On=2e3,vo=2001,Bn=class{addEventListener(t,e){this._listeners===void 0&&(this._listeners={});let i=this._listeners;i[t]===void 0&&(i[t]=[]),i[t].indexOf(e)===-1&&i[t].push(e)}hasEventListener(t,e){if(this._listeners===void 0)return!1;let i=this._listeners;return i[t]!==void 0&&i[t].indexOf(e)!==-1}removeEventListener(t,e){if(this._listeners===void 0)return;let s=this._listeners[t];if(s!==void 0){let r=s.indexOf(e);r!==-1&&s.splice(r,1)}}dispatchEvent(t){if(this._listeners===void 0)return;let i=this._listeners[t.type];if(i!==void 0){t.target=this;let s=i.slice(0);for(let r=0,o=s.length;r<o;r++)s[r].call(this,t);t.target=null}}},Ce=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],qh=1234567,sr=Math.PI/180,cr=180/Math.PI;function Es(){let n=Math.random()*4294967295|0,t=Math.random()*4294967295|0,e=Math.random()*4294967295|0,i=Math.random()*4294967295|0;return(Ce[n&255]+Ce[n>>8&255]+Ce[n>>16&255]+Ce[n>>24&255]+"-"+Ce[t&255]+Ce[t>>8&255]+"-"+Ce[t>>16&15|64]+Ce[t>>24&255]+"-"+Ce[e&63|128]+Ce[e>>8&255]+"-"+Ce[e>>16&255]+Ce[e>>24&255]+Ce[i&255]+Ce[i>>8&255]+Ce[i>>16&255]+Ce[i>>24&255]).toLowerCase()}function Ue(n,t,e){return Math.max(t,Math.min(e,n))}function Vl(n,t){return(n%t+t)%t}function pp(n,t,e,i,s){return i+(n-t)*(s-i)/(e-t)}function mp(n,t,e){return n!==t?(e-n)/(t-n):0}function rr(n,t,e){return(1-e)*n+e*t}function gp(n,t,e,i){return rr(n,t,1-Math.exp(-e*i))}function xp(n,t=1){return t-Math.abs(Vl(n,t*2)-t)}function vp(n,t,e){return n<=t?0:n>=e?1:(n=(n-t)/(e-t),n*n*(3-2*n))}function _p(n,t,e){return n<=t?0:n>=e?1:(n=(n-t)/(e-t),n*n*n*(n*(n*6-15)+10))}function yp(n,t){return n+Math.floor(Math.random()*(t-n+1))}function Mp(n,t){return n+Math.random()*(t-n)}function bp(n){return n*(.5-Math.random())}function Sp(n){n!==void 0&&(qh=n);let t=qh+=1831565813;return t=Math.imul(t^t>>>15,t|1),t^=t+Math.imul(t^t>>>7,t|61),((t^t>>>14)>>>0)/4294967296}function wp(n){return n*sr}function Ap(n){return n*cr}function Ep(n){return(n&n-1)===0&&n!==0}function Tp(n){return Math.pow(2,Math.ceil(Math.log(n)/Math.LN2))}function Cp(n){return Math.pow(2,Math.floor(Math.log(n)/Math.LN2))}function Rp(n,t,e,i,s){let r=Math.cos,o=Math.sin,a=r(e/2),c=o(e/2),l=r((t+i)/2),f=o((t+i)/2),d=r((t-i)/2),h=o((t-i)/2),m=r((i-t)/2),x=o((i-t)/2);switch(s){case"XYX":n.set(a*f,c*d,c*h,a*l);break;case"YZY":n.set(c*h,a*f,c*d,a*l);break;case"ZXZ":n.set(c*d,c*h,a*f,a*l);break;case"XZX":n.set(a*f,c*x,c*m,a*l);break;case"YXY":n.set(c*m,a*f,c*x,a*l);break;case"ZYZ":n.set(c*x,c*m,a*f,a*l);break;default:console.warn("THREE.MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+s)}}function us(n,t){switch(t.constructor){case Float32Array:return n;case Uint32Array:return n/4294967295;case Uint16Array:return n/65535;case Uint8Array:return n/255;case Int32Array:return Math.max(n/2147483647,-1);case Int16Array:return Math.max(n/32767,-1);case Int8Array:return Math.max(n/127,-1);default:throw new Error("Invalid component type.")}}function Le(n,t){switch(t.constructor){case Float32Array:return n;case Uint32Array:return Math.round(n*4294967295);case Uint16Array:return Math.round(n*65535);case Uint8Array:return Math.round(n*255);case Int32Array:return Math.round(n*2147483647);case Int16Array:return Math.round(n*32767);case Int8Array:return Math.round(n*127);default:throw new Error("Invalid component type.")}}var Gl={DEG2RAD:sr,RAD2DEG:cr,generateUUID:Es,clamp:Ue,euclideanModulo:Vl,mapLinear:pp,inverseLerp:mp,lerp:rr,damp:gp,pingpong:xp,smoothstep:vp,smootherstep:_p,randInt:yp,randFloat:Mp,randFloatSpread:bp,seededRandom:Sp,degToRad:wp,radToDeg:Ap,isPowerOfTwo:Ep,ceilPowerOfTwo:Tp,floorPowerOfTwo:Cp,setQuaternionFromProperEuler:Rp,normalize:Le,denormalize:us},yt=class n{constructor(t=0,e=0){n.prototype.isVector2=!0,this.x=t,this.y=e}get width(){return this.x}set width(t){this.x=t}get height(){return this.y}set height(t){this.y=t}set(t,e){return this.x=t,this.y=e,this}setScalar(t){return this.x=t,this.y=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y)}copy(t){return this.x=t.x,this.y=t.y,this}add(t){return this.x+=t.x,this.y+=t.y,this}addScalar(t){return this.x+=t,this.y+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this}subScalar(t){return this.x-=t,this.y-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this}multiply(t){return this.x*=t.x,this.y*=t.y,this}multiplyScalar(t){return this.x*=t,this.y*=t,this}divide(t){return this.x/=t.x,this.y/=t.y,this}divideScalar(t){return this.multiplyScalar(1/t)}applyMatrix3(t){let e=this.x,i=this.y,s=t.elements;return this.x=s[0]*e+s[3]*i+s[6],this.y=s[1]*e+s[4]*i+s[7],this}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(t){return this.x*t.x+this.y*t.y}cross(t){return this.x*t.y-this.y*t.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(t){let e=Math.sqrt(this.lengthSq()*t.lengthSq());if(e===0)return Math.PI/2;let i=this.dot(t)/e;return Math.acos(Ue(i,-1,1))}distanceTo(t){return Math.sqrt(this.distanceToSquared(t))}distanceToSquared(t){let e=this.x-t.x,i=this.y-t.y;return e*e+i*i}manhattanDistanceTo(t){return Math.abs(this.x-t.x)+Math.abs(this.y-t.y)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this}equals(t){return t.x===this.x&&t.y===this.y}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this}rotateAround(t,e){let i=Math.cos(e),s=Math.sin(e),r=this.x-t.x,o=this.y-t.y;return this.x=r*i-o*s+t.x,this.y=r*s+o*i+t.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},zt=class n{constructor(t,e,i,s,r,o,a,c,l){n.prototype.isMatrix3=!0,this.elements=[1,0,0,0,1,0,0,0,1],t!==void 0&&this.set(t,e,i,s,r,o,a,c,l)}set(t,e,i,s,r,o,a,c,l){let f=this.elements;return f[0]=t,f[1]=s,f[2]=a,f[3]=e,f[4]=r,f[5]=c,f[6]=i,f[7]=o,f[8]=l,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(t){let e=this.elements,i=t.elements;return e[0]=i[0],e[1]=i[1],e[2]=i[2],e[3]=i[3],e[4]=i[4],e[5]=i[5],e[6]=i[6],e[7]=i[7],e[8]=i[8],this}extractBasis(t,e,i){return t.setFromMatrix3Column(this,0),e.setFromMatrix3Column(this,1),i.setFromMatrix3Column(this,2),this}setFromMatrix4(t){let e=t.elements;return this.set(e[0],e[4],e[8],e[1],e[5],e[9],e[2],e[6],e[10]),this}multiply(t){return this.multiplyMatrices(this,t)}premultiply(t){return this.multiplyMatrices(t,this)}multiplyMatrices(t,e){let i=t.elements,s=e.elements,r=this.elements,o=i[0],a=i[3],c=i[6],l=i[1],f=i[4],d=i[7],h=i[2],m=i[5],x=i[8],_=s[0],u=s[3],p=s[6],b=s[1],y=s[4],A=s[7],R=s[2],T=s[5],E=s[8];return r[0]=o*_+a*b+c*R,r[3]=o*u+a*y+c*T,r[6]=o*p+a*A+c*E,r[1]=l*_+f*b+d*R,r[4]=l*u+f*y+d*T,r[7]=l*p+f*A+d*E,r[2]=h*_+m*b+x*R,r[5]=h*u+m*y+x*T,r[8]=h*p+m*A+x*E,this}multiplyScalar(t){let e=this.elements;return e[0]*=t,e[3]*=t,e[6]*=t,e[1]*=t,e[4]*=t,e[7]*=t,e[2]*=t,e[5]*=t,e[8]*=t,this}determinant(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],c=t[6],l=t[7],f=t[8];return e*o*f-e*a*l-i*r*f+i*a*c+s*r*l-s*o*c}invert(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],c=t[6],l=t[7],f=t[8],d=f*o-a*l,h=a*c-f*r,m=l*r-o*c,x=e*d+i*h+s*m;if(x===0)return this.set(0,0,0,0,0,0,0,0,0);let _=1/x;return t[0]=d*_,t[1]=(s*l-f*i)*_,t[2]=(a*i-s*o)*_,t[3]=h*_,t[4]=(f*e-s*c)*_,t[5]=(s*r-a*e)*_,t[6]=m*_,t[7]=(i*c-l*e)*_,t[8]=(o*e-i*r)*_,this}transpose(){let t,e=this.elements;return t=e[1],e[1]=e[3],e[3]=t,t=e[2],e[2]=e[6],e[6]=t,t=e[5],e[5]=e[7],e[7]=t,this}getNormalMatrix(t){return this.setFromMatrix4(t).invert().transpose()}transposeIntoArray(t){let e=this.elements;return t[0]=e[0],t[1]=e[3],t[2]=e[6],t[3]=e[1],t[4]=e[4],t[5]=e[7],t[6]=e[2],t[7]=e[5],t[8]=e[8],this}setUvTransform(t,e,i,s,r,o,a){let c=Math.cos(r),l=Math.sin(r);return this.set(i*c,i*l,-i*(c*o+l*a)+o+t,-s*l,s*c,-s*(-l*o+c*a)+a+e,0,0,1),this}scale(t,e){return this.premultiply(Ga.makeScale(t,e)),this}rotate(t){return this.premultiply(Ga.makeRotation(-t)),this}translate(t,e){return this.premultiply(Ga.makeTranslation(t,e)),this}makeTranslation(t,e){return t.isVector2?this.set(1,0,t.x,0,1,t.y,0,0,1):this.set(1,0,t,0,1,e,0,0,1),this}makeRotation(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,-i,0,i,e,0,0,0,1),this}makeScale(t,e){return this.set(t,0,0,0,e,0,0,0,1),this}equals(t){let e=this.elements,i=t.elements;for(let s=0;s<9;s++)if(e[s]!==i[s])return!1;return!0}fromArray(t,e=0){for(let i=0;i<9;i++)this.elements[i]=t[i+e];return this}toArray(t=[],e=0){let i=this.elements;return t[e]=i[0],t[e+1]=i[1],t[e+2]=i[2],t[e+3]=i[3],t[e+4]=i[4],t[e+5]=i[5],t[e+6]=i[6],t[e+7]=i[7],t[e+8]=i[8],t}clone(){return new this.constructor().fromArray(this.elements)}},Ga=new zt;function td(n){for(let t=n.length-1;t>=0;--t)if(n[t]>=65535)return!0;return!1}function _o(n){return document.createElementNS("http://www.w3.org/1999/xhtml",n)}function Pp(){let n=_o("canvas");return n.style.display="block",n}var Yh={};function fo(n){n in Yh||(Yh[n]=!0,console.warn(n))}function Ip(n,t,e){return new Promise(function(i,s){function r(){switch(n.clientWaitSync(t,n.SYNC_FLUSH_COMMANDS_BIT,0)){case n.WAIT_FAILED:s();break;case n.TIMEOUT_EXPIRED:setTimeout(r,e);break;default:i()}}setTimeout(r,e)})}function Lp(n){let t=n.elements;t[2]=.5*t[2]+.5*t[3],t[6]=.5*t[6]+.5*t[7],t[10]=.5*t[10]+.5*t[11],t[14]=.5*t[14]+.5*t[15]}function Dp(n){let t=n.elements;t[11]===-1?(t[10]=-t[10]-1,t[14]=-t[14]):(t[10]=-t[10],t[14]=-t[14]+1)}var Zh=new zt().set(.8224621,.177538,0,.0331941,.9668058,0,.0170827,.0723974,.9105199),jh=new zt().set(1.2249401,-.2249404,0,-.0420569,1.0420571,0,-.0196376,-.0786361,1.0982735),Ks={[si]:{transfer:mo,primaries:go,luminanceCoefficients:[.2126,.7152,.0722],toReference:n=>n,fromReference:n=>n},[yn]:{transfer:ie,primaries:go,luminanceCoefficients:[.2126,.7152,.0722],toReference:n=>n.convertSRGBToLinear(),fromReference:n=>n.convertLinearToSRGB()},[Vo]:{transfer:mo,primaries:xo,luminanceCoefficients:[.2289,.6917,.0793],toReference:n=>n.applyMatrix3(jh),fromReference:n=>n.applyMatrix3(Zh)},[Hl]:{transfer:ie,primaries:xo,luminanceCoefficients:[.2289,.6917,.0793],toReference:n=>n.convertSRGBToLinear().applyMatrix3(jh),fromReference:n=>n.applyMatrix3(Zh).convertLinearToSRGB()}},Up=new Set([si,Vo]),Jt={enabled:!0,_workingColorSpace:si,get workingColorSpace(){return this._workingColorSpace},set workingColorSpace(n){if(!Up.has(n))throw new Error(`Unsupported working color space, "${n}".`);this._workingColorSpace=n},convert:function(n,t,e){if(this.enabled===!1||t===e||!t||!e)return n;let i=Ks[t].toReference,s=Ks[e].fromReference;return s(i(n))},fromWorkingColorSpace:function(n,t){return this.convert(n,this._workingColorSpace,t)},toWorkingColorSpace:function(n,t){return this.convert(n,t,this._workingColorSpace)},getPrimaries:function(n){return Ks[n].primaries},getTransfer:function(n){return n===Qn?mo:Ks[n].transfer},getLuminanceCoefficients:function(n,t=this._workingColorSpace){return n.fromArray(Ks[t].luminanceCoefficients)}};function ms(n){return n<.04045?n*.0773993808:Math.pow(n*.9478672986+.0521327014,2.4)}function Wa(n){return n<.0031308?n*12.92:1.055*Math.pow(n,.41666)-.055}var Ji,il=class{static getDataURL(t){if(/^data:/i.test(t.src)||typeof HTMLCanvasElement>"u")return t.src;let e;if(t instanceof HTMLCanvasElement)e=t;else{Ji===void 0&&(Ji=_o("canvas")),Ji.width=t.width,Ji.height=t.height;let i=Ji.getContext("2d");t instanceof ImageData?i.putImageData(t,0,0):i.drawImage(t,0,0,t.width,t.height),e=Ji}return e.width>2048||e.height>2048?(console.warn("THREE.ImageUtils.getDataURL: Image converted to jpg for performance reasons",t),e.toDataURL("image/jpeg",.6)):e.toDataURL("image/png")}static sRGBToLinear(t){if(typeof HTMLImageElement<"u"&&t instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&t instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&t instanceof ImageBitmap){let e=_o("canvas");e.width=t.width,e.height=t.height;let i=e.getContext("2d");i.drawImage(t,0,0,t.width,t.height);let s=i.getImageData(0,0,t.width,t.height),r=s.data;for(let o=0;o<r.length;o++)r[o]=ms(r[o]/255)*255;return i.putImageData(s,0,0),e}else if(t.data){let e=t.data.slice(0);for(let i=0;i<e.length;i++)e instanceof Uint8Array||e instanceof Uint8ClampedArray?e[i]=Math.floor(ms(e[i]/255)*255):e[i]=ms(e[i]);return{data:e,width:t.width,height:t.height}}else return console.warn("THREE.ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),t}},Np=0,yo=class{constructor(t=null){this.isSource=!0,Object.defineProperty(this,"id",{value:Np++}),this.uuid=Es(),this.data=t,this.dataReady=!0,this.version=0}set needsUpdate(t){t===!0&&this.version++}toJSON(t){let e=t===void 0||typeof t=="string";if(!e&&t.images[this.uuid]!==void 0)return t.images[this.uuid];let i={uuid:this.uuid,url:""},s=this.data;if(s!==null){let r;if(Array.isArray(s)){r=[];for(let o=0,a=s.length;o<a;o++)s[o].isDataTexture?r.push(Xa(s[o].image)):r.push(Xa(s[o]))}else r=Xa(s);i.url=r}return e||(t.images[this.uuid]=i),i}};function Xa(n){return typeof HTMLImageElement<"u"&&n instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&n instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&n instanceof ImageBitmap?il.getDataURL(n):n.data?{data:Array.from(n.data),width:n.width,height:n.height,type:n.data.constructor.name}:(console.warn("THREE.Texture: Unable to serialize Texture."),{})}var Op=0,Pe=class n extends Bn{constructor(t=n.DEFAULT_IMAGE,e=n.DEFAULT_MAPPING,i=Ai,s=Ai,r=je,o=Ei,a=fn,c=Fn,l=n.DEFAULT_ANISOTROPY,f=Qn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:Op++}),this.uuid=Es(),this.name="",this.source=new yo(t),this.mipmaps=[],this.mapping=e,this.channel=0,this.wrapS=i,this.wrapT=s,this.magFilter=r,this.minFilter=o,this.anisotropy=l,this.format=a,this.internalFormat=null,this.type=c,this.offset=new yt(0,0),this.repeat=new yt(1,1),this.center=new yt(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new zt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=f,this.userData={},this.version=0,this.onUpdate=null,this.isRenderTargetTexture=!1,this.pmremVersion=0}get image(){return this.source.data}set image(t=null){this.source.data=t}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}clone(){return new this.constructor().copy(this)}copy(t){return this.name=t.name,this.source=t.source,this.mipmaps=t.mipmaps.slice(0),this.mapping=t.mapping,this.channel=t.channel,this.wrapS=t.wrapS,this.wrapT=t.wrapT,this.magFilter=t.magFilter,this.minFilter=t.minFilter,this.anisotropy=t.anisotropy,this.format=t.format,this.internalFormat=t.internalFormat,this.type=t.type,this.offset.copy(t.offset),this.repeat.copy(t.repeat),this.center.copy(t.center),this.rotation=t.rotation,this.matrixAutoUpdate=t.matrixAutoUpdate,this.matrix.copy(t.matrix),this.generateMipmaps=t.generateMipmaps,this.premultiplyAlpha=t.premultiplyAlpha,this.flipY=t.flipY,this.unpackAlignment=t.unpackAlignment,this.colorSpace=t.colorSpace,this.userData=JSON.parse(JSON.stringify(t.userData)),this.needsUpdate=!0,this}toJSON(t){let e=t===void 0||typeof t=="string";if(!e&&t.textures[this.uuid]!==void 0)return t.textures[this.uuid];let i={metadata:{version:4.6,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(t).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(i.userData=this.userData),e||(t.textures[this.uuid]=i),i}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(t){if(this.mapping!==Vu)return t;if(t.applyMatrix3(this.matrix),t.x<0||t.x>1)switch(this.wrapS){case Cc:t.x=t.x-Math.floor(t.x);break;case Ai:t.x=t.x<0?0:1;break;case Rc:Math.abs(Math.floor(t.x)%2)===1?t.x=Math.ceil(t.x)-t.x:t.x=t.x-Math.floor(t.x);break}if(t.y<0||t.y>1)switch(this.wrapT){case Cc:t.y=t.y-Math.floor(t.y);break;case Ai:t.y=t.y<0?0:1;break;case Rc:Math.abs(Math.floor(t.y)%2)===1?t.y=Math.ceil(t.y)-t.y:t.y=t.y-Math.floor(t.y);break}return this.flipY&&(t.y=1-t.y),t}set needsUpdate(t){t===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(t){t===!0&&this.pmremVersion++}};Pe.DEFAULT_IMAGE=null;Pe.DEFAULT_MAPPING=Vu;Pe.DEFAULT_ANISOTROPY=1;var ce=class n{constructor(t=0,e=0,i=0,s=1){n.prototype.isVector4=!0,this.x=t,this.y=e,this.z=i,this.w=s}get width(){return this.z}set width(t){this.z=t}get height(){return this.w}set height(t){this.w=t}set(t,e,i,s){return this.x=t,this.y=e,this.z=i,this.w=s,this}setScalar(t){return this.x=t,this.y=t,this.z=t,this.w=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setZ(t){return this.z=t,this}setW(t){return this.w=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;case 2:this.z=e;break;case 3:this.w=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(t){return this.x=t.x,this.y=t.y,this.z=t.z,this.w=t.w!==void 0?t.w:1,this}add(t){return this.x+=t.x,this.y+=t.y,this.z+=t.z,this.w+=t.w,this}addScalar(t){return this.x+=t,this.y+=t,this.z+=t,this.w+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this.z=t.z+e.z,this.w=t.w+e.w,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this.z+=t.z*e,this.w+=t.w*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this.z-=t.z,this.w-=t.w,this}subScalar(t){return this.x-=t,this.y-=t,this.z-=t,this.w-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this.z=t.z-e.z,this.w=t.w-e.w,this}multiply(t){return this.x*=t.x,this.y*=t.y,this.z*=t.z,this.w*=t.w,this}multiplyScalar(t){return this.x*=t,this.y*=t,this.z*=t,this.w*=t,this}applyMatrix4(t){let e=this.x,i=this.y,s=this.z,r=this.w,o=t.elements;return this.x=o[0]*e+o[4]*i+o[8]*s+o[12]*r,this.y=o[1]*e+o[5]*i+o[9]*s+o[13]*r,this.z=o[2]*e+o[6]*i+o[10]*s+o[14]*r,this.w=o[3]*e+o[7]*i+o[11]*s+o[15]*r,this}divideScalar(t){return this.multiplyScalar(1/t)}setAxisAngleFromQuaternion(t){this.w=2*Math.acos(t.w);let e=Math.sqrt(1-t.w*t.w);return e<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=t.x/e,this.y=t.y/e,this.z=t.z/e),this}setAxisAngleFromRotationMatrix(t){let e,i,s,r,c=t.elements,l=c[0],f=c[4],d=c[8],h=c[1],m=c[5],x=c[9],_=c[2],u=c[6],p=c[10];if(Math.abs(f-h)<.01&&Math.abs(d-_)<.01&&Math.abs(x-u)<.01){if(Math.abs(f+h)<.1&&Math.abs(d+_)<.1&&Math.abs(x+u)<.1&&Math.abs(l+m+p-3)<.1)return this.set(1,0,0,0),this;e=Math.PI;let y=(l+1)/2,A=(m+1)/2,R=(p+1)/2,T=(f+h)/4,E=(d+_)/4,C=(x+u)/4;return y>A&&y>R?y<.01?(i=0,s=.707106781,r=.707106781):(i=Math.sqrt(y),s=T/i,r=E/i):A>R?A<.01?(i=.707106781,s=0,r=.707106781):(s=Math.sqrt(A),i=T/s,r=C/s):R<.01?(i=.707106781,s=.707106781,r=0):(r=Math.sqrt(R),i=E/r,s=C/r),this.set(i,s,r,e),this}let b=Math.sqrt((u-x)*(u-x)+(d-_)*(d-_)+(h-f)*(h-f));return Math.abs(b)<.001&&(b=1),this.x=(u-x)/b,this.y=(d-_)/b,this.z=(h-f)/b,this.w=Math.acos((l+m+p-1)/2),this}setFromMatrixPosition(t){let e=t.elements;return this.x=e[12],this.y=e[13],this.z=e[14],this.w=e[15],this}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this.z=Math.min(this.z,t.z),this.w=Math.min(this.w,t.w),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this.z=Math.max(this.z,t.z),this.w=Math.max(this.w,t.w),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this.z=Math.max(t.z,Math.min(e.z,this.z)),this.w=Math.max(t.w,Math.min(e.w,this.w)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this.z=Math.max(t,Math.min(e,this.z)),this.w=Math.max(t,Math.min(e,this.w)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(t){return this.x*t.x+this.y*t.y+this.z*t.z+this.w*t.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this.z+=(t.z-this.z)*e,this.w+=(t.w-this.w)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this.z=t.z+(e.z-t.z)*i,this.w=t.w+(e.w-t.w)*i,this}equals(t){return t.x===this.x&&t.y===this.y&&t.z===this.z&&t.w===this.w}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this.z=t[e+2],this.w=t[e+3],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t[e+2]=this.z,t[e+3]=this.w,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this.z=t.getZ(e),this.w=t.getW(e),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},sl=class extends Bn{constructor(t=1,e=1,i={}){super(),this.isRenderTarget=!0,this.width=t,this.height=e,this.depth=1,this.scissor=new ce(0,0,t,e),this.scissorTest=!1,this.viewport=new ce(0,0,t,e);let s={width:t,height:e,depth:1};i=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:je,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1},i);let r=new Pe(s,i.mapping,i.wrapS,i.wrapT,i.magFilter,i.minFilter,i.format,i.type,i.anisotropy,i.colorSpace);r.flipY=!1,r.generateMipmaps=i.generateMipmaps,r.internalFormat=i.internalFormat,this.textures=[];let o=i.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0;this.depthBuffer=i.depthBuffer,this.stencilBuffer=i.stencilBuffer,this.resolveDepthBuffer=i.resolveDepthBuffer,this.resolveStencilBuffer=i.resolveStencilBuffer,this.depthTexture=i.depthTexture,this.samples=i.samples}get texture(){return this.textures[0]}set texture(t){this.textures[0]=t}setSize(t,e,i=1){if(this.width!==t||this.height!==e||this.depth!==i){this.width=t,this.height=e,this.depth=i;for(let s=0,r=this.textures.length;s<r;s++)this.textures[s].image.width=t,this.textures[s].image.height=e,this.textures[s].image.depth=i;this.dispose()}this.viewport.set(0,0,t,e),this.scissor.set(0,0,t,e)}clone(){return new this.constructor().copy(this)}copy(t){this.width=t.width,this.height=t.height,this.depth=t.depth,this.scissor.copy(t.scissor),this.scissorTest=t.scissorTest,this.viewport.copy(t.viewport),this.textures.length=0;for(let i=0,s=t.textures.length;i<s;i++)this.textures[i]=t.textures[i].clone(),this.textures[i].isRenderTargetTexture=!0;let e=Object.assign({},t.texture.image);return this.texture.source=new yo(e),this.depthBuffer=t.depthBuffer,this.stencilBuffer=t.stencilBuffer,this.resolveDepthBuffer=t.resolveDepthBuffer,this.resolveStencilBuffer=t.resolveStencilBuffer,t.depthTexture!==null&&(this.depthTexture=t.depthTexture.clone()),this.samples=t.samples,this}dispose(){this.dispatchEvent({type:"dispose"})}},fe=class extends sl{constructor(t=1,e=1,i={}){super(t,e,i),this.isWebGLRenderTarget=!0}},Mo=class extends Pe{constructor(t=null,e=1,i=1,s=1){super(null),this.isDataArrayTexture=!0,this.image={data:t,width:e,height:i,depth:s},this.magFilter=Ee,this.minFilter=Ee,this.wrapR=Ai,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(t){this.layerUpdates.add(t)}clearLayerUpdates(){this.layerUpdates.clear()}};var rl=class extends Pe{constructor(t=null,e=1,i=1,s=1){super(null),this.isData3DTexture=!0,this.image={data:t,width:e,height:i,depth:s},this.magFilter=Ee,this.minFilter=Ee,this.wrapR=Ai,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var ke=class{constructor(t=0,e=0,i=0,s=1){this.isQuaternion=!0,this._x=t,this._y=e,this._z=i,this._w=s}static slerpFlat(t,e,i,s,r,o,a){let c=i[s+0],l=i[s+1],f=i[s+2],d=i[s+3],h=r[o+0],m=r[o+1],x=r[o+2],_=r[o+3];if(a===0){t[e+0]=c,t[e+1]=l,t[e+2]=f,t[e+3]=d;return}if(a===1){t[e+0]=h,t[e+1]=m,t[e+2]=x,t[e+3]=_;return}if(d!==_||c!==h||l!==m||f!==x){let u=1-a,p=c*h+l*m+f*x+d*_,b=p>=0?1:-1,y=1-p*p;if(y>Number.EPSILON){let R=Math.sqrt(y),T=Math.atan2(R,p*b);u=Math.sin(u*T)/R,a=Math.sin(a*T)/R}let A=a*b;if(c=c*u+h*A,l=l*u+m*A,f=f*u+x*A,d=d*u+_*A,u===1-a){let R=1/Math.sqrt(c*c+l*l+f*f+d*d);c*=R,l*=R,f*=R,d*=R}}t[e]=c,t[e+1]=l,t[e+2]=f,t[e+3]=d}static multiplyQuaternionsFlat(t,e,i,s,r,o){let a=i[s],c=i[s+1],l=i[s+2],f=i[s+3],d=r[o],h=r[o+1],m=r[o+2],x=r[o+3];return t[e]=a*x+f*d+c*m-l*h,t[e+1]=c*x+f*h+l*d-a*m,t[e+2]=l*x+f*m+a*h-c*d,t[e+3]=f*x-a*d-c*h-l*m,t}get x(){return this._x}set x(t){this._x=t,this._onChangeCallback()}get y(){return this._y}set y(t){this._y=t,this._onChangeCallback()}get z(){return this._z}set z(t){this._z=t,this._onChangeCallback()}get w(){return this._w}set w(t){this._w=t,this._onChangeCallback()}set(t,e,i,s){return this._x=t,this._y=e,this._z=i,this._w=s,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(t){return this._x=t.x,this._y=t.y,this._z=t.z,this._w=t.w,this._onChangeCallback(),this}setFromEuler(t,e=!0){let i=t._x,s=t._y,r=t._z,o=t._order,a=Math.cos,c=Math.sin,l=a(i/2),f=a(s/2),d=a(r/2),h=c(i/2),m=c(s/2),x=c(r/2);switch(o){case"XYZ":this._x=h*f*d+l*m*x,this._y=l*m*d-h*f*x,this._z=l*f*x+h*m*d,this._w=l*f*d-h*m*x;break;case"YXZ":this._x=h*f*d+l*m*x,this._y=l*m*d-h*f*x,this._z=l*f*x-h*m*d,this._w=l*f*d+h*m*x;break;case"ZXY":this._x=h*f*d-l*m*x,this._y=l*m*d+h*f*x,this._z=l*f*x+h*m*d,this._w=l*f*d-h*m*x;break;case"ZYX":this._x=h*f*d-l*m*x,this._y=l*m*d+h*f*x,this._z=l*f*x-h*m*d,this._w=l*f*d+h*m*x;break;case"YZX":this._x=h*f*d+l*m*x,this._y=l*m*d+h*f*x,this._z=l*f*x-h*m*d,this._w=l*f*d-h*m*x;break;case"XZY":this._x=h*f*d-l*m*x,this._y=l*m*d-h*f*x,this._z=l*f*x+h*m*d,this._w=l*f*d+h*m*x;break;default:console.warn("THREE.Quaternion: .setFromEuler() encountered an unknown order: "+o)}return e===!0&&this._onChangeCallback(),this}setFromAxisAngle(t,e){let i=e/2,s=Math.sin(i);return this._x=t.x*s,this._y=t.y*s,this._z=t.z*s,this._w=Math.cos(i),this._onChangeCallback(),this}setFromRotationMatrix(t){let e=t.elements,i=e[0],s=e[4],r=e[8],o=e[1],a=e[5],c=e[9],l=e[2],f=e[6],d=e[10],h=i+a+d;if(h>0){let m=.5/Math.sqrt(h+1);this._w=.25/m,this._x=(f-c)*m,this._y=(r-l)*m,this._z=(o-s)*m}else if(i>a&&i>d){let m=2*Math.sqrt(1+i-a-d);this._w=(f-c)/m,this._x=.25*m,this._y=(s+o)/m,this._z=(r+l)/m}else if(a>d){let m=2*Math.sqrt(1+a-i-d);this._w=(r-l)/m,this._x=(s+o)/m,this._y=.25*m,this._z=(c+f)/m}else{let m=2*Math.sqrt(1+d-i-a);this._w=(o-s)/m,this._x=(r+l)/m,this._y=(c+f)/m,this._z=.25*m}return this._onChangeCallback(),this}setFromUnitVectors(t,e){let i=t.dot(e)+1;return i<Number.EPSILON?(i=0,Math.abs(t.x)>Math.abs(t.z)?(this._x=-t.y,this._y=t.x,this._z=0,this._w=i):(this._x=0,this._y=-t.z,this._z=t.y,this._w=i)):(this._x=t.y*e.z-t.z*e.y,this._y=t.z*e.x-t.x*e.z,this._z=t.x*e.y-t.y*e.x,this._w=i),this.normalize()}angleTo(t){return 2*Math.acos(Math.abs(Ue(this.dot(t),-1,1)))}rotateTowards(t,e){let i=this.angleTo(t);if(i===0)return this;let s=Math.min(1,e/i);return this.slerp(t,s),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(t){return this._x*t._x+this._y*t._y+this._z*t._z+this._w*t._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let t=this.length();return t===0?(this._x=0,this._y=0,this._z=0,this._w=1):(t=1/t,this._x=this._x*t,this._y=this._y*t,this._z=this._z*t,this._w=this._w*t),this._onChangeCallback(),this}multiply(t){return this.multiplyQuaternions(this,t)}premultiply(t){return this.multiplyQuaternions(t,this)}multiplyQuaternions(t,e){let i=t._x,s=t._y,r=t._z,o=t._w,a=e._x,c=e._y,l=e._z,f=e._w;return this._x=i*f+o*a+s*l-r*c,this._y=s*f+o*c+r*a-i*l,this._z=r*f+o*l+i*c-s*a,this._w=o*f-i*a-s*c-r*l,this._onChangeCallback(),this}slerp(t,e){if(e===0)return this;if(e===1)return this.copy(t);let i=this._x,s=this._y,r=this._z,o=this._w,a=o*t._w+i*t._x+s*t._y+r*t._z;if(a<0?(this._w=-t._w,this._x=-t._x,this._y=-t._y,this._z=-t._z,a=-a):this.copy(t),a>=1)return this._w=o,this._x=i,this._y=s,this._z=r,this;let c=1-a*a;if(c<=Number.EPSILON){let m=1-e;return this._w=m*o+e*this._w,this._x=m*i+e*this._x,this._y=m*s+e*this._y,this._z=m*r+e*this._z,this.normalize(),this}let l=Math.sqrt(c),f=Math.atan2(l,a),d=Math.sin((1-e)*f)/l,h=Math.sin(e*f)/l;return this._w=o*d+this._w*h,this._x=i*d+this._x*h,this._y=s*d+this._y*h,this._z=r*d+this._z*h,this._onChangeCallback(),this}slerpQuaternions(t,e,i){return this.copy(t).slerp(e,i)}random(){let t=2*Math.PI*Math.random(),e=2*Math.PI*Math.random(),i=Math.random(),s=Math.sqrt(1-i),r=Math.sqrt(i);return this.set(s*Math.sin(t),s*Math.cos(t),r*Math.sin(e),r*Math.cos(e))}equals(t){return t._x===this._x&&t._y===this._y&&t._z===this._z&&t._w===this._w}fromArray(t,e=0){return this._x=t[e],this._y=t[e+1],this._z=t[e+2],this._w=t[e+3],this._onChangeCallback(),this}toArray(t=[],e=0){return t[e]=this._x,t[e+1]=this._y,t[e+2]=this._z,t[e+3]=this._w,t}fromBufferAttribute(t,e){return this._x=t.getX(e),this._y=t.getY(e),this._z=t.getZ(e),this._w=t.getW(e),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(t){return this._onChangeCallback=t,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},D=class n{constructor(t=0,e=0,i=0){n.prototype.isVector3=!0,this.x=t,this.y=e,this.z=i}set(t,e,i){return i===void 0&&(i=this.z),this.x=t,this.y=e,this.z=i,this}setScalar(t){return this.x=t,this.y=t,this.z=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setZ(t){return this.z=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;case 2:this.z=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(t){return this.x=t.x,this.y=t.y,this.z=t.z,this}add(t){return this.x+=t.x,this.y+=t.y,this.z+=t.z,this}addScalar(t){return this.x+=t,this.y+=t,this.z+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this.z=t.z+e.z,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this.z+=t.z*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this.z-=t.z,this}subScalar(t){return this.x-=t,this.y-=t,this.z-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this.z=t.z-e.z,this}multiply(t){return this.x*=t.x,this.y*=t.y,this.z*=t.z,this}multiplyScalar(t){return this.x*=t,this.y*=t,this.z*=t,this}multiplyVectors(t,e){return this.x=t.x*e.x,this.y=t.y*e.y,this.z=t.z*e.z,this}applyEuler(t){return this.applyQuaternion(Kh.setFromEuler(t))}applyAxisAngle(t,e){return this.applyQuaternion(Kh.setFromAxisAngle(t,e))}applyMatrix3(t){let e=this.x,i=this.y,s=this.z,r=t.elements;return this.x=r[0]*e+r[3]*i+r[6]*s,this.y=r[1]*e+r[4]*i+r[7]*s,this.z=r[2]*e+r[5]*i+r[8]*s,this}applyNormalMatrix(t){return this.applyMatrix3(t).normalize()}applyMatrix4(t){let e=this.x,i=this.y,s=this.z,r=t.elements,o=1/(r[3]*e+r[7]*i+r[11]*s+r[15]);return this.x=(r[0]*e+r[4]*i+r[8]*s+r[12])*o,this.y=(r[1]*e+r[5]*i+r[9]*s+r[13])*o,this.z=(r[2]*e+r[6]*i+r[10]*s+r[14])*o,this}applyQuaternion(t){let e=this.x,i=this.y,s=this.z,r=t.x,o=t.y,a=t.z,c=t.w,l=2*(o*s-a*i),f=2*(a*e-r*s),d=2*(r*i-o*e);return this.x=e+c*l+o*d-a*f,this.y=i+c*f+a*l-r*d,this.z=s+c*d+r*f-o*l,this}project(t){return this.applyMatrix4(t.matrixWorldInverse).applyMatrix4(t.projectionMatrix)}unproject(t){return this.applyMatrix4(t.projectionMatrixInverse).applyMatrix4(t.matrixWorld)}transformDirection(t){let e=this.x,i=this.y,s=this.z,r=t.elements;return this.x=r[0]*e+r[4]*i+r[8]*s,this.y=r[1]*e+r[5]*i+r[9]*s,this.z=r[2]*e+r[6]*i+r[10]*s,this.normalize()}divide(t){return this.x/=t.x,this.y/=t.y,this.z/=t.z,this}divideScalar(t){return this.multiplyScalar(1/t)}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this.z=Math.min(this.z,t.z),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this.z=Math.max(this.z,t.z),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this.z=Math.max(t.z,Math.min(e.z,this.z)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this.z=Math.max(t,Math.min(e,this.z)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(t){return this.x*t.x+this.y*t.y+this.z*t.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this.z+=(t.z-this.z)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this.z=t.z+(e.z-t.z)*i,this}cross(t){return this.crossVectors(this,t)}crossVectors(t,e){let i=t.x,s=t.y,r=t.z,o=e.x,a=e.y,c=e.z;return this.x=s*c-r*a,this.y=r*o-i*c,this.z=i*a-s*o,this}projectOnVector(t){let e=t.lengthSq();if(e===0)return this.set(0,0,0);let i=t.dot(this)/e;return this.copy(t).multiplyScalar(i)}projectOnPlane(t){return qa.copy(this).projectOnVector(t),this.sub(qa)}reflect(t){return this.sub(qa.copy(t).multiplyScalar(2*this.dot(t)))}angleTo(t){let e=Math.sqrt(this.lengthSq()*t.lengthSq());if(e===0)return Math.PI/2;let i=this.dot(t)/e;return Math.acos(Ue(i,-1,1))}distanceTo(t){return Math.sqrt(this.distanceToSquared(t))}distanceToSquared(t){let e=this.x-t.x,i=this.y-t.y,s=this.z-t.z;return e*e+i*i+s*s}manhattanDistanceTo(t){return Math.abs(this.x-t.x)+Math.abs(this.y-t.y)+Math.abs(this.z-t.z)}setFromSpherical(t){return this.setFromSphericalCoords(t.radius,t.phi,t.theta)}setFromSphericalCoords(t,e,i){let s=Math.sin(e)*t;return this.x=s*Math.sin(i),this.y=Math.cos(e)*t,this.z=s*Math.cos(i),this}setFromCylindrical(t){return this.setFromCylindricalCoords(t.radius,t.theta,t.y)}setFromCylindricalCoords(t,e,i){return this.x=t*Math.sin(e),this.y=i,this.z=t*Math.cos(e),this}setFromMatrixPosition(t){let e=t.elements;return this.x=e[12],this.y=e[13],this.z=e[14],this}setFromMatrixScale(t){let e=this.setFromMatrixColumn(t,0).length(),i=this.setFromMatrixColumn(t,1).length(),s=this.setFromMatrixColumn(t,2).length();return this.x=e,this.y=i,this.z=s,this}setFromMatrixColumn(t,e){return this.fromArray(t.elements,e*4)}setFromMatrix3Column(t,e){return this.fromArray(t.elements,e*3)}setFromEuler(t){return this.x=t._x,this.y=t._y,this.z=t._z,this}setFromColor(t){return this.x=t.r,this.y=t.g,this.z=t.b,this}equals(t){return t.x===this.x&&t.y===this.y&&t.z===this.z}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this.z=t[e+2],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t[e+2]=this.z,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this.z=t.getZ(e),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let t=Math.random()*Math.PI*2,e=Math.random()*2-1,i=Math.sqrt(1-e*e);return this.x=i*Math.cos(t),this.y=e,this.z=i*Math.sin(t),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},qa=new D,Kh=new ke,zn=class{constructor(t=new D(1/0,1/0,1/0),e=new D(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=t,this.max=e}set(t,e){return this.min.copy(t),this.max.copy(e),this}setFromArray(t){this.makeEmpty();for(let e=0,i=t.length;e<i;e+=3)this.expandByPoint(ln.fromArray(t,e));return this}setFromBufferAttribute(t){this.makeEmpty();for(let e=0,i=t.count;e<i;e++)this.expandByPoint(ln.fromBufferAttribute(t,e));return this}setFromPoints(t){this.makeEmpty();for(let e=0,i=t.length;e<i;e++)this.expandByPoint(t[e]);return this}setFromCenterAndSize(t,e){let i=ln.copy(e).multiplyScalar(.5);return this.min.copy(t).sub(i),this.max.copy(t).add(i),this}setFromObject(t,e=!1){return this.makeEmpty(),this.expandByObject(t,e)}clone(){return new this.constructor().copy(this)}copy(t){return this.min.copy(t.min),this.max.copy(t.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(t){return this.isEmpty()?t.set(0,0,0):t.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(t){return this.isEmpty()?t.set(0,0,0):t.subVectors(this.max,this.min)}expandByPoint(t){return this.min.min(t),this.max.max(t),this}expandByVector(t){return this.min.sub(t),this.max.add(t),this}expandByScalar(t){return this.min.addScalar(-t),this.max.addScalar(t),this}expandByObject(t,e=!1){t.updateWorldMatrix(!1,!1);let i=t.geometry;if(i!==void 0){let r=i.getAttribute("position");if(e===!0&&r!==void 0&&t.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)t.isMesh===!0?t.getVertexPosition(o,ln):ln.fromBufferAttribute(r,o),ln.applyMatrix4(t.matrixWorld),this.expandByPoint(ln);else t.boundingBox!==void 0?(t.boundingBox===null&&t.computeBoundingBox(),zr.copy(t.boundingBox)):(i.boundingBox===null&&i.computeBoundingBox(),zr.copy(i.boundingBox)),zr.applyMatrix4(t.matrixWorld),this.union(zr)}let s=t.children;for(let r=0,o=s.length;r<o;r++)this.expandByObject(s[r],e);return this}containsPoint(t){return t.x>=this.min.x&&t.x<=this.max.x&&t.y>=this.min.y&&t.y<=this.max.y&&t.z>=this.min.z&&t.z<=this.max.z}containsBox(t){return this.min.x<=t.min.x&&t.max.x<=this.max.x&&this.min.y<=t.min.y&&t.max.y<=this.max.y&&this.min.z<=t.min.z&&t.max.z<=this.max.z}getParameter(t,e){return e.set((t.x-this.min.x)/(this.max.x-this.min.x),(t.y-this.min.y)/(this.max.y-this.min.y),(t.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(t){return t.max.x>=this.min.x&&t.min.x<=this.max.x&&t.max.y>=this.min.y&&t.min.y<=this.max.y&&t.max.z>=this.min.z&&t.min.z<=this.max.z}intersectsSphere(t){return this.clampPoint(t.center,ln),ln.distanceToSquared(t.center)<=t.radius*t.radius}intersectsPlane(t){let e,i;return t.normal.x>0?(e=t.normal.x*this.min.x,i=t.normal.x*this.max.x):(e=t.normal.x*this.max.x,i=t.normal.x*this.min.x),t.normal.y>0?(e+=t.normal.y*this.min.y,i+=t.normal.y*this.max.y):(e+=t.normal.y*this.max.y,i+=t.normal.y*this.min.y),t.normal.z>0?(e+=t.normal.z*this.min.z,i+=t.normal.z*this.max.z):(e+=t.normal.z*this.max.z,i+=t.normal.z*this.min.z),e<=-t.constant&&i>=-t.constant}intersectsTriangle(t){if(this.isEmpty())return!1;this.getCenter(Js),kr.subVectors(this.max,Js),Qi.subVectors(t.a,Js),$i.subVectors(t.b,Js),ts.subVectors(t.c,Js),qn.subVectors($i,Qi),Yn.subVectors(ts,$i),mi.subVectors(Qi,ts);let e=[0,-qn.z,qn.y,0,-Yn.z,Yn.y,0,-mi.z,mi.y,qn.z,0,-qn.x,Yn.z,0,-Yn.x,mi.z,0,-mi.x,-qn.y,qn.x,0,-Yn.y,Yn.x,0,-mi.y,mi.x,0];return!Ya(e,Qi,$i,ts,kr)||(e=[1,0,0,0,1,0,0,0,1],!Ya(e,Qi,$i,ts,kr))?!1:(Hr.crossVectors(qn,Yn),e=[Hr.x,Hr.y,Hr.z],Ya(e,Qi,$i,ts,kr))}clampPoint(t,e){return e.copy(t).clamp(this.min,this.max)}distanceToPoint(t){return this.clampPoint(t,ln).distanceTo(t)}getBoundingSphere(t){return this.isEmpty()?t.makeEmpty():(this.getCenter(t.center),t.radius=this.getSize(ln).length()*.5),t}intersect(t){return this.min.max(t.min),this.max.min(t.max),this.isEmpty()&&this.makeEmpty(),this}union(t){return this.min.min(t.min),this.max.max(t.max),this}applyMatrix4(t){return this.isEmpty()?this:(Rn[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(t),Rn[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(t),Rn[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(t),Rn[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(t),Rn[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(t),Rn[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(t),Rn[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(t),Rn[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(t),this.setFromPoints(Rn),this)}translate(t){return this.min.add(t),this.max.add(t),this}equals(t){return t.min.equals(this.min)&&t.max.equals(this.max)}},Rn=[new D,new D,new D,new D,new D,new D,new D,new D],ln=new D,zr=new zn,Qi=new D,$i=new D,ts=new D,qn=new D,Yn=new D,mi=new D,Js=new D,kr=new D,Hr=new D,gi=new D;function Ya(n,t,e,i,s){for(let r=0,o=n.length-3;r<=o;r+=3){gi.fromArray(n,r);let a=s.x*Math.abs(gi.x)+s.y*Math.abs(gi.y)+s.z*Math.abs(gi.z),c=t.dot(gi),l=e.dot(gi),f=i.dot(gi);if(Math.max(-Math.max(c,l,f),Math.min(c,l,f))>a)return!1}return!0}var Fp=new zn,Qs=new D,Za=new D,Ci=class{constructor(t=new D,e=-1){this.isSphere=!0,this.center=t,this.radius=e}set(t,e){return this.center.copy(t),this.radius=e,this}setFromPoints(t,e){let i=this.center;e!==void 0?i.copy(e):Fp.setFromPoints(t).getCenter(i);let s=0;for(let r=0,o=t.length;r<o;r++)s=Math.max(s,i.distanceToSquared(t[r]));return this.radius=Math.sqrt(s),this}copy(t){return this.center.copy(t.center),this.radius=t.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(t){return t.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(t){return t.distanceTo(this.center)-this.radius}intersectsSphere(t){let e=this.radius+t.radius;return t.center.distanceToSquared(this.center)<=e*e}intersectsBox(t){return t.intersectsSphere(this)}intersectsPlane(t){return Math.abs(t.distanceToPoint(this.center))<=this.radius}clampPoint(t,e){let i=this.center.distanceToSquared(t);return e.copy(t),i>this.radius*this.radius&&(e.sub(this.center).normalize(),e.multiplyScalar(this.radius).add(this.center)),e}getBoundingBox(t){return this.isEmpty()?(t.makeEmpty(),t):(t.set(this.center,this.center),t.expandByScalar(this.radius),t)}applyMatrix4(t){return this.center.applyMatrix4(t),this.radius=this.radius*t.getMaxScaleOnAxis(),this}translate(t){return this.center.add(t),this}expandByPoint(t){if(this.isEmpty())return this.center.copy(t),this.radius=0,this;Qs.subVectors(t,this.center);let e=Qs.lengthSq();if(e>this.radius*this.radius){let i=Math.sqrt(e),s=(i-this.radius)*.5;this.center.addScaledVector(Qs,s/i),this.radius+=s}return this}union(t){return t.isEmpty()?this:this.isEmpty()?(this.copy(t),this):(this.center.equals(t.center)===!0?this.radius=Math.max(this.radius,t.radius):(Za.subVectors(t.center,this.center).setLength(t.radius),this.expandByPoint(Qs.copy(t.center).add(Za)),this.expandByPoint(Qs.copy(t.center).sub(Za))),this)}equals(t){return t.center.equals(this.center)&&t.radius===this.radius}clone(){return new this.constructor().copy(this)}},Pn=new D,ja=new D,Vr=new D,Zn=new D,Ka=new D,Gr=new D,Ja=new D,bo=class{constructor(t=new D,e=new D(0,0,-1)){this.origin=t,this.direction=e}set(t,e){return this.origin.copy(t),this.direction.copy(e),this}copy(t){return this.origin.copy(t.origin),this.direction.copy(t.direction),this}at(t,e){return e.copy(this.origin).addScaledVector(this.direction,t)}lookAt(t){return this.direction.copy(t).sub(this.origin).normalize(),this}recast(t){return this.origin.copy(this.at(t,Pn)),this}closestPointToPoint(t,e){e.subVectors(t,this.origin);let i=e.dot(this.direction);return i<0?e.copy(this.origin):e.copy(this.origin).addScaledVector(this.direction,i)}distanceToPoint(t){return Math.sqrt(this.distanceSqToPoint(t))}distanceSqToPoint(t){let e=Pn.subVectors(t,this.origin).dot(this.direction);return e<0?this.origin.distanceToSquared(t):(Pn.copy(this.origin).addScaledVector(this.direction,e),Pn.distanceToSquared(t))}distanceSqToSegment(t,e,i,s){ja.copy(t).add(e).multiplyScalar(.5),Vr.copy(e).sub(t).normalize(),Zn.copy(this.origin).sub(ja);let r=t.distanceTo(e)*.5,o=-this.direction.dot(Vr),a=Zn.dot(this.direction),c=-Zn.dot(Vr),l=Zn.lengthSq(),f=Math.abs(1-o*o),d,h,m,x;if(f>0)if(d=o*c-a,h=o*a-c,x=r*f,d>=0)if(h>=-x)if(h<=x){let _=1/f;d*=_,h*=_,m=d*(d+o*h+2*a)+h*(o*d+h+2*c)+l}else h=r,d=Math.max(0,-(o*h+a)),m=-d*d+h*(h+2*c)+l;else h=-r,d=Math.max(0,-(o*h+a)),m=-d*d+h*(h+2*c)+l;else h<=-x?(d=Math.max(0,-(-o*r+a)),h=d>0?-r:Math.min(Math.max(-r,-c),r),m=-d*d+h*(h+2*c)+l):h<=x?(d=0,h=Math.min(Math.max(-r,-c),r),m=h*(h+2*c)+l):(d=Math.max(0,-(o*r+a)),h=d>0?r:Math.min(Math.max(-r,-c),r),m=-d*d+h*(h+2*c)+l);else h=o>0?-r:r,d=Math.max(0,-(o*h+a)),m=-d*d+h*(h+2*c)+l;return i&&i.copy(this.origin).addScaledVector(this.direction,d),s&&s.copy(ja).addScaledVector(Vr,h),m}intersectSphere(t,e){Pn.subVectors(t.center,this.origin);let i=Pn.dot(this.direction),s=Pn.dot(Pn)-i*i,r=t.radius*t.radius;if(s>r)return null;let o=Math.sqrt(r-s),a=i-o,c=i+o;return c<0?null:a<0?this.at(c,e):this.at(a,e)}intersectsSphere(t){return this.distanceSqToPoint(t.center)<=t.radius*t.radius}distanceToPlane(t){let e=t.normal.dot(this.direction);if(e===0)return t.distanceToPoint(this.origin)===0?0:null;let i=-(this.origin.dot(t.normal)+t.constant)/e;return i>=0?i:null}intersectPlane(t,e){let i=this.distanceToPlane(t);return i===null?null:this.at(i,e)}intersectsPlane(t){let e=t.distanceToPoint(this.origin);return e===0||t.normal.dot(this.direction)*e<0}intersectBox(t,e){let i,s,r,o,a,c,l=1/this.direction.x,f=1/this.direction.y,d=1/this.direction.z,h=this.origin;return l>=0?(i=(t.min.x-h.x)*l,s=(t.max.x-h.x)*l):(i=(t.max.x-h.x)*l,s=(t.min.x-h.x)*l),f>=0?(r=(t.min.y-h.y)*f,o=(t.max.y-h.y)*f):(r=(t.max.y-h.y)*f,o=(t.min.y-h.y)*f),i>o||r>s||((r>i||isNaN(i))&&(i=r),(o<s||isNaN(s))&&(s=o),d>=0?(a=(t.min.z-h.z)*d,c=(t.max.z-h.z)*d):(a=(t.max.z-h.z)*d,c=(t.min.z-h.z)*d),i>c||a>s)||((a>i||i!==i)&&(i=a),(c<s||s!==s)&&(s=c),s<0)?null:this.at(i>=0?i:s,e)}intersectsBox(t){return this.intersectBox(t,Pn)!==null}intersectTriangle(t,e,i,s,r){Ka.subVectors(e,t),Gr.subVectors(i,t),Ja.crossVectors(Ka,Gr);let o=this.direction.dot(Ja),a;if(o>0){if(s)return null;a=1}else if(o<0)a=-1,o=-o;else return null;Zn.subVectors(this.origin,t);let c=a*this.direction.dot(Gr.crossVectors(Zn,Gr));if(c<0)return null;let l=a*this.direction.dot(Ka.cross(Zn));if(l<0||c+l>o)return null;let f=-a*Zn.dot(Ja);return f<0?null:this.at(f/o,r)}applyMatrix4(t){return this.origin.applyMatrix4(t),this.direction.transformDirection(t),this}equals(t){return t.origin.equals(this.origin)&&t.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},$t=class n{constructor(t,e,i,s,r,o,a,c,l,f,d,h,m,x,_,u){n.prototype.isMatrix4=!0,this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],t!==void 0&&this.set(t,e,i,s,r,o,a,c,l,f,d,h,m,x,_,u)}set(t,e,i,s,r,o,a,c,l,f,d,h,m,x,_,u){let p=this.elements;return p[0]=t,p[4]=e,p[8]=i,p[12]=s,p[1]=r,p[5]=o,p[9]=a,p[13]=c,p[2]=l,p[6]=f,p[10]=d,p[14]=h,p[3]=m,p[7]=x,p[11]=_,p[15]=u,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new n().fromArray(this.elements)}copy(t){let e=this.elements,i=t.elements;return e[0]=i[0],e[1]=i[1],e[2]=i[2],e[3]=i[3],e[4]=i[4],e[5]=i[5],e[6]=i[6],e[7]=i[7],e[8]=i[8],e[9]=i[9],e[10]=i[10],e[11]=i[11],e[12]=i[12],e[13]=i[13],e[14]=i[14],e[15]=i[15],this}copyPosition(t){let e=this.elements,i=t.elements;return e[12]=i[12],e[13]=i[13],e[14]=i[14],this}setFromMatrix3(t){let e=t.elements;return this.set(e[0],e[3],e[6],0,e[1],e[4],e[7],0,e[2],e[5],e[8],0,0,0,0,1),this}extractBasis(t,e,i){return t.setFromMatrixColumn(this,0),e.setFromMatrixColumn(this,1),i.setFromMatrixColumn(this,2),this}makeBasis(t,e,i){return this.set(t.x,e.x,i.x,0,t.y,e.y,i.y,0,t.z,e.z,i.z,0,0,0,0,1),this}extractRotation(t){let e=this.elements,i=t.elements,s=1/es.setFromMatrixColumn(t,0).length(),r=1/es.setFromMatrixColumn(t,1).length(),o=1/es.setFromMatrixColumn(t,2).length();return e[0]=i[0]*s,e[1]=i[1]*s,e[2]=i[2]*s,e[3]=0,e[4]=i[4]*r,e[5]=i[5]*r,e[6]=i[6]*r,e[7]=0,e[8]=i[8]*o,e[9]=i[9]*o,e[10]=i[10]*o,e[11]=0,e[12]=0,e[13]=0,e[14]=0,e[15]=1,this}makeRotationFromEuler(t){let e=this.elements,i=t.x,s=t.y,r=t.z,o=Math.cos(i),a=Math.sin(i),c=Math.cos(s),l=Math.sin(s),f=Math.cos(r),d=Math.sin(r);if(t.order==="XYZ"){let h=o*f,m=o*d,x=a*f,_=a*d;e[0]=c*f,e[4]=-c*d,e[8]=l,e[1]=m+x*l,e[5]=h-_*l,e[9]=-a*c,e[2]=_-h*l,e[6]=x+m*l,e[10]=o*c}else if(t.order==="YXZ"){let h=c*f,m=c*d,x=l*f,_=l*d;e[0]=h+_*a,e[4]=x*a-m,e[8]=o*l,e[1]=o*d,e[5]=o*f,e[9]=-a,e[2]=m*a-x,e[6]=_+h*a,e[10]=o*c}else if(t.order==="ZXY"){let h=c*f,m=c*d,x=l*f,_=l*d;e[0]=h-_*a,e[4]=-o*d,e[8]=x+m*a,e[1]=m+x*a,e[5]=o*f,e[9]=_-h*a,e[2]=-o*l,e[6]=a,e[10]=o*c}else if(t.order==="ZYX"){let h=o*f,m=o*d,x=a*f,_=a*d;e[0]=c*f,e[4]=x*l-m,e[8]=h*l+_,e[1]=c*d,e[5]=_*l+h,e[9]=m*l-x,e[2]=-l,e[6]=a*c,e[10]=o*c}else if(t.order==="YZX"){let h=o*c,m=o*l,x=a*c,_=a*l;e[0]=c*f,e[4]=_-h*d,e[8]=x*d+m,e[1]=d,e[5]=o*f,e[9]=-a*f,e[2]=-l*f,e[6]=m*d+x,e[10]=h-_*d}else if(t.order==="XZY"){let h=o*c,m=o*l,x=a*c,_=a*l;e[0]=c*f,e[4]=-d,e[8]=l*f,e[1]=h*d+_,e[5]=o*f,e[9]=m*d-x,e[2]=x*d-m,e[6]=a*f,e[10]=_*d+h}return e[3]=0,e[7]=0,e[11]=0,e[12]=0,e[13]=0,e[14]=0,e[15]=1,this}makeRotationFromQuaternion(t){return this.compose(Bp,t,zp)}lookAt(t,e,i){let s=this.elements;return Ye.subVectors(t,e),Ye.lengthSq()===0&&(Ye.z=1),Ye.normalize(),jn.crossVectors(i,Ye),jn.lengthSq()===0&&(Math.abs(i.z)===1?Ye.x+=1e-4:Ye.z+=1e-4,Ye.normalize(),jn.crossVectors(i,Ye)),jn.normalize(),Wr.crossVectors(Ye,jn),s[0]=jn.x,s[4]=Wr.x,s[8]=Ye.x,s[1]=jn.y,s[5]=Wr.y,s[9]=Ye.y,s[2]=jn.z,s[6]=Wr.z,s[10]=Ye.z,this}multiply(t){return this.multiplyMatrices(this,t)}premultiply(t){return this.multiplyMatrices(t,this)}multiplyMatrices(t,e){let i=t.elements,s=e.elements,r=this.elements,o=i[0],a=i[4],c=i[8],l=i[12],f=i[1],d=i[5],h=i[9],m=i[13],x=i[2],_=i[6],u=i[10],p=i[14],b=i[3],y=i[7],A=i[11],R=i[15],T=s[0],E=s[4],C=s[8],N=s[12],g=s[1],S=s[5],F=s[9],V=s[13],q=s[2],et=s[6],B=s[10],it=s[14],Z=s[3],gt=s[7],_t=s[11],bt=s[15];return r[0]=o*T+a*g+c*q+l*Z,r[4]=o*E+a*S+c*et+l*gt,r[8]=o*C+a*F+c*B+l*_t,r[12]=o*N+a*V+c*it+l*bt,r[1]=f*T+d*g+h*q+m*Z,r[5]=f*E+d*S+h*et+m*gt,r[9]=f*C+d*F+h*B+m*_t,r[13]=f*N+d*V+h*it+m*bt,r[2]=x*T+_*g+u*q+p*Z,r[6]=x*E+_*S+u*et+p*gt,r[10]=x*C+_*F+u*B+p*_t,r[14]=x*N+_*V+u*it+p*bt,r[3]=b*T+y*g+A*q+R*Z,r[7]=b*E+y*S+A*et+R*gt,r[11]=b*C+y*F+A*B+R*_t,r[15]=b*N+y*V+A*it+R*bt,this}multiplyScalar(t){let e=this.elements;return e[0]*=t,e[4]*=t,e[8]*=t,e[12]*=t,e[1]*=t,e[5]*=t,e[9]*=t,e[13]*=t,e[2]*=t,e[6]*=t,e[10]*=t,e[14]*=t,e[3]*=t,e[7]*=t,e[11]*=t,e[15]*=t,this}determinant(){let t=this.elements,e=t[0],i=t[4],s=t[8],r=t[12],o=t[1],a=t[5],c=t[9],l=t[13],f=t[2],d=t[6],h=t[10],m=t[14],x=t[3],_=t[7],u=t[11],p=t[15];return x*(+r*c*d-s*l*d-r*a*h+i*l*h+s*a*m-i*c*m)+_*(+e*c*m-e*l*h+r*o*h-s*o*m+s*l*f-r*c*f)+u*(+e*l*d-e*a*m-r*o*d+i*o*m+r*a*f-i*l*f)+p*(-s*a*f-e*c*d+e*a*h+s*o*d-i*o*h+i*c*f)}transpose(){let t=this.elements,e;return e=t[1],t[1]=t[4],t[4]=e,e=t[2],t[2]=t[8],t[8]=e,e=t[6],t[6]=t[9],t[9]=e,e=t[3],t[3]=t[12],t[12]=e,e=t[7],t[7]=t[13],t[13]=e,e=t[11],t[11]=t[14],t[14]=e,this}setPosition(t,e,i){let s=this.elements;return t.isVector3?(s[12]=t.x,s[13]=t.y,s[14]=t.z):(s[12]=t,s[13]=e,s[14]=i),this}invert(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],c=t[6],l=t[7],f=t[8],d=t[9],h=t[10],m=t[11],x=t[12],_=t[13],u=t[14],p=t[15],b=d*u*l-_*h*l+_*c*m-a*u*m-d*c*p+a*h*p,y=x*h*l-f*u*l-x*c*m+o*u*m+f*c*p-o*h*p,A=f*_*l-x*d*l+x*a*m-o*_*m-f*a*p+o*d*p,R=x*d*c-f*_*c-x*a*h+o*_*h+f*a*u-o*d*u,T=e*b+i*y+s*A+r*R;if(T===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let E=1/T;return t[0]=b*E,t[1]=(_*h*r-d*u*r-_*s*m+i*u*m+d*s*p-i*h*p)*E,t[2]=(a*u*r-_*c*r+_*s*l-i*u*l-a*s*p+i*c*p)*E,t[3]=(d*c*r-a*h*r-d*s*l+i*h*l+a*s*m-i*c*m)*E,t[4]=y*E,t[5]=(f*u*r-x*h*r+x*s*m-e*u*m-f*s*p+e*h*p)*E,t[6]=(x*c*r-o*u*r-x*s*l+e*u*l+o*s*p-e*c*p)*E,t[7]=(o*h*r-f*c*r+f*s*l-e*h*l-o*s*m+e*c*m)*E,t[8]=A*E,t[9]=(x*d*r-f*_*r-x*i*m+e*_*m+f*i*p-e*d*p)*E,t[10]=(o*_*r-x*a*r+x*i*l-e*_*l-o*i*p+e*a*p)*E,t[11]=(f*a*r-o*d*r-f*i*l+e*d*l+o*i*m-e*a*m)*E,t[12]=R*E,t[13]=(f*_*s-x*d*s+x*i*h-e*_*h-f*i*u+e*d*u)*E,t[14]=(x*a*s-o*_*s-x*i*c+e*_*c+o*i*u-e*a*u)*E,t[15]=(o*d*s-f*a*s+f*i*c-e*d*c-o*i*h+e*a*h)*E,this}scale(t){let e=this.elements,i=t.x,s=t.y,r=t.z;return e[0]*=i,e[4]*=s,e[8]*=r,e[1]*=i,e[5]*=s,e[9]*=r,e[2]*=i,e[6]*=s,e[10]*=r,e[3]*=i,e[7]*=s,e[11]*=r,this}getMaxScaleOnAxis(){let t=this.elements,e=t[0]*t[0]+t[1]*t[1]+t[2]*t[2],i=t[4]*t[4]+t[5]*t[5]+t[6]*t[6],s=t[8]*t[8]+t[9]*t[9]+t[10]*t[10];return Math.sqrt(Math.max(e,i,s))}makeTranslation(t,e,i){return t.isVector3?this.set(1,0,0,t.x,0,1,0,t.y,0,0,1,t.z,0,0,0,1):this.set(1,0,0,t,0,1,0,e,0,0,1,i,0,0,0,1),this}makeRotationX(t){let e=Math.cos(t),i=Math.sin(t);return this.set(1,0,0,0,0,e,-i,0,0,i,e,0,0,0,0,1),this}makeRotationY(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,0,i,0,0,1,0,0,-i,0,e,0,0,0,0,1),this}makeRotationZ(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,-i,0,0,i,e,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(t,e){let i=Math.cos(e),s=Math.sin(e),r=1-i,o=t.x,a=t.y,c=t.z,l=r*o,f=r*a;return this.set(l*o+i,l*a-s*c,l*c+s*a,0,l*a+s*c,f*a+i,f*c-s*o,0,l*c-s*a,f*c+s*o,r*c*c+i,0,0,0,0,1),this}makeScale(t,e,i){return this.set(t,0,0,0,0,e,0,0,0,0,i,0,0,0,0,1),this}makeShear(t,e,i,s,r,o){return this.set(1,i,r,0,t,1,o,0,e,s,1,0,0,0,0,1),this}compose(t,e,i){let s=this.elements,r=e._x,o=e._y,a=e._z,c=e._w,l=r+r,f=o+o,d=a+a,h=r*l,m=r*f,x=r*d,_=o*f,u=o*d,p=a*d,b=c*l,y=c*f,A=c*d,R=i.x,T=i.y,E=i.z;return s[0]=(1-(_+p))*R,s[1]=(m+A)*R,s[2]=(x-y)*R,s[3]=0,s[4]=(m-A)*T,s[5]=(1-(h+p))*T,s[6]=(u+b)*T,s[7]=0,s[8]=(x+y)*E,s[9]=(u-b)*E,s[10]=(1-(h+_))*E,s[11]=0,s[12]=t.x,s[13]=t.y,s[14]=t.z,s[15]=1,this}decompose(t,e,i){let s=this.elements,r=es.set(s[0],s[1],s[2]).length(),o=es.set(s[4],s[5],s[6]).length(),a=es.set(s[8],s[9],s[10]).length();this.determinant()<0&&(r=-r),t.x=s[12],t.y=s[13],t.z=s[14],hn.copy(this);let l=1/r,f=1/o,d=1/a;return hn.elements[0]*=l,hn.elements[1]*=l,hn.elements[2]*=l,hn.elements[4]*=f,hn.elements[5]*=f,hn.elements[6]*=f,hn.elements[8]*=d,hn.elements[9]*=d,hn.elements[10]*=d,e.setFromRotationMatrix(hn),i.x=r,i.y=o,i.z=a,this}makePerspective(t,e,i,s,r,o,a=On){let c=this.elements,l=2*r/(e-t),f=2*r/(i-s),d=(e+t)/(e-t),h=(i+s)/(i-s),m,x;if(a===On)m=-(o+r)/(o-r),x=-2*o*r/(o-r);else if(a===vo)m=-o/(o-r),x=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=l,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=f,c[9]=h,c[13]=0,c[2]=0,c[6]=0,c[10]=m,c[14]=x,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(t,e,i,s,r,o,a=On){let c=this.elements,l=1/(e-t),f=1/(i-s),d=1/(o-r),h=(e+t)*l,m=(i+s)*f,x,_;if(a===On)x=(o+r)*d,_=-2*d;else if(a===vo)x=r*d,_=-1*d;else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=2*l,c[4]=0,c[8]=0,c[12]=-h,c[1]=0,c[5]=2*f,c[9]=0,c[13]=-m,c[2]=0,c[6]=0,c[10]=_,c[14]=-x,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(t){let e=this.elements,i=t.elements;for(let s=0;s<16;s++)if(e[s]!==i[s])return!1;return!0}fromArray(t,e=0){for(let i=0;i<16;i++)this.elements[i]=t[i+e];return this}toArray(t=[],e=0){let i=this.elements;return t[e]=i[0],t[e+1]=i[1],t[e+2]=i[2],t[e+3]=i[3],t[e+4]=i[4],t[e+5]=i[5],t[e+6]=i[6],t[e+7]=i[7],t[e+8]=i[8],t[e+9]=i[9],t[e+10]=i[10],t[e+11]=i[11],t[e+12]=i[12],t[e+13]=i[13],t[e+14]=i[14],t[e+15]=i[15],t}},es=new D,hn=new $t,Bp=new D(0,0,0),zp=new D(1,1,1),jn=new D,Wr=new D,Ye=new D,Jh=new $t,Qh=new ke,nn=class n{constructor(t=0,e=0,i=0,s=n.DEFAULT_ORDER){this.isEuler=!0,this._x=t,this._y=e,this._z=i,this._order=s}get x(){return this._x}set x(t){this._x=t,this._onChangeCallback()}get y(){return this._y}set y(t){this._y=t,this._onChangeCallback()}get z(){return this._z}set z(t){this._z=t,this._onChangeCallback()}get order(){return this._order}set order(t){this._order=t,this._onChangeCallback()}set(t,e,i,s=this._order){return this._x=t,this._y=e,this._z=i,this._order=s,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(t){return this._x=t._x,this._y=t._y,this._z=t._z,this._order=t._order,this._onChangeCallback(),this}setFromRotationMatrix(t,e=this._order,i=!0){let s=t.elements,r=s[0],o=s[4],a=s[8],c=s[1],l=s[5],f=s[9],d=s[2],h=s[6],m=s[10];switch(e){case"XYZ":this._y=Math.asin(Ue(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-f,m),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(h,l),this._z=0);break;case"YXZ":this._x=Math.asin(-Ue(f,-1,1)),Math.abs(f)<.9999999?(this._y=Math.atan2(a,m),this._z=Math.atan2(c,l)):(this._y=Math.atan2(-d,r),this._z=0);break;case"ZXY":this._x=Math.asin(Ue(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(-d,m),this._z=Math.atan2(-o,l)):(this._y=0,this._z=Math.atan2(c,r));break;case"ZYX":this._y=Math.asin(-Ue(d,-1,1)),Math.abs(d)<.9999999?(this._x=Math.atan2(h,m),this._z=Math.atan2(c,r)):(this._x=0,this._z=Math.atan2(-o,l));break;case"YZX":this._z=Math.asin(Ue(c,-1,1)),Math.abs(c)<.9999999?(this._x=Math.atan2(-f,l),this._y=Math.atan2(-d,r)):(this._x=0,this._y=Math.atan2(a,m));break;case"XZY":this._z=Math.asin(-Ue(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(h,l),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-f,m),this._y=0);break;default:console.warn("THREE.Euler: .setFromRotationMatrix() encountered an unknown order: "+e)}return this._order=e,i===!0&&this._onChangeCallback(),this}setFromQuaternion(t,e,i){return Jh.makeRotationFromQuaternion(t),this.setFromRotationMatrix(Jh,e,i)}setFromVector3(t,e=this._order){return this.set(t.x,t.y,t.z,e)}reorder(t){return Qh.setFromEuler(this),this.setFromQuaternion(Qh,t)}equals(t){return t._x===this._x&&t._y===this._y&&t._z===this._z&&t._order===this._order}fromArray(t){return this._x=t[0],this._y=t[1],this._z=t[2],t[3]!==void 0&&(this._order=t[3]),this._onChangeCallback(),this}toArray(t=[],e=0){return t[e]=this._x,t[e+1]=this._y,t[e+2]=this._z,t[e+3]=this._order,t}_onChange(t){return this._onChangeCallback=t,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};nn.DEFAULT_ORDER="XYZ";var lr=class{constructor(){this.mask=1}set(t){this.mask=(1<<t|0)>>>0}enable(t){this.mask|=1<<t|0}enableAll(){this.mask=-1}toggle(t){this.mask^=1<<t|0}disable(t){this.mask&=~(1<<t|0)}disableAll(){this.mask=0}test(t){return(this.mask&t.mask)!==0}isEnabled(t){return(this.mask&(1<<t|0))!==0}},kp=0,$h=new D,ns=new ke,In=new $t,Xr=new D,$s=new D,Hp=new D,Vp=new ke,tu=new D(1,0,0),eu=new D(0,1,0),nu=new D(0,0,1),iu={type:"added"},Gp={type:"removed"},is={type:"childadded",child:null},Qa={type:"childremoved",child:null},He=class n extends Bn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:kp++}),this.uuid=Es(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=n.DEFAULT_UP.clone();let t=new D,e=new nn,i=new ke,s=new D(1,1,1);function r(){i.setFromEuler(e,!1)}function o(){e.setFromQuaternion(i,void 0,!1)}e._onChange(r),i._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:t},rotation:{configurable:!0,enumerable:!0,value:e},quaternion:{configurable:!0,enumerable:!0,value:i},scale:{configurable:!0,enumerable:!0,value:s},modelViewMatrix:{value:new $t},normalMatrix:{value:new zt}}),this.matrix=new $t,this.matrixWorld=new $t,this.matrixAutoUpdate=n.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=n.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new lr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.userData={}}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(t){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(t),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(t){return this.quaternion.premultiply(t),this}setRotationFromAxisAngle(t,e){this.quaternion.setFromAxisAngle(t,e)}setRotationFromEuler(t){this.quaternion.setFromEuler(t,!0)}setRotationFromMatrix(t){this.quaternion.setFromRotationMatrix(t)}setRotationFromQuaternion(t){this.quaternion.copy(t)}rotateOnAxis(t,e){return ns.setFromAxisAngle(t,e),this.quaternion.multiply(ns),this}rotateOnWorldAxis(t,e){return ns.setFromAxisAngle(t,e),this.quaternion.premultiply(ns),this}rotateX(t){return this.rotateOnAxis(tu,t)}rotateY(t){return this.rotateOnAxis(eu,t)}rotateZ(t){return this.rotateOnAxis(nu,t)}translateOnAxis(t,e){return $h.copy(t).applyQuaternion(this.quaternion),this.position.add($h.multiplyScalar(e)),this}translateX(t){return this.translateOnAxis(tu,t)}translateY(t){return this.translateOnAxis(eu,t)}translateZ(t){return this.translateOnAxis(nu,t)}localToWorld(t){return this.updateWorldMatrix(!0,!1),t.applyMatrix4(this.matrixWorld)}worldToLocal(t){return this.updateWorldMatrix(!0,!1),t.applyMatrix4(In.copy(this.matrixWorld).invert())}lookAt(t,e,i){t.isVector3?Xr.copy(t):Xr.set(t,e,i);let s=this.parent;this.updateWorldMatrix(!0,!1),$s.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?In.lookAt($s,Xr,this.up):In.lookAt(Xr,$s,this.up),this.quaternion.setFromRotationMatrix(In),s&&(In.extractRotation(s.matrixWorld),ns.setFromRotationMatrix(In),this.quaternion.premultiply(ns.invert()))}add(t){if(arguments.length>1){for(let e=0;e<arguments.length;e++)this.add(arguments[e]);return this}return t===this?(console.error("THREE.Object3D.add: object can't be added as a child of itself.",t),this):(t&&t.isObject3D?(t.removeFromParent(),t.parent=this,this.children.push(t),t.dispatchEvent(iu),is.child=t,this.dispatchEvent(is),is.child=null):console.error("THREE.Object3D.add: object not an instance of THREE.Object3D.",t),this)}remove(t){if(arguments.length>1){for(let i=0;i<arguments.length;i++)this.remove(arguments[i]);return this}let e=this.children.indexOf(t);return e!==-1&&(t.parent=null,this.children.splice(e,1),t.dispatchEvent(Gp),Qa.child=t,this.dispatchEvent(Qa),Qa.child=null),this}removeFromParent(){let t=this.parent;return t!==null&&t.remove(this),this}clear(){return this.remove(...this.children)}attach(t){return this.updateWorldMatrix(!0,!1),In.copy(this.matrixWorld).invert(),t.parent!==null&&(t.parent.updateWorldMatrix(!0,!1),In.multiply(t.parent.matrixWorld)),t.applyMatrix4(In),t.removeFromParent(),t.parent=this,this.children.push(t),t.updateWorldMatrix(!1,!0),t.dispatchEvent(iu),is.child=t,this.dispatchEvent(is),is.child=null,this}getObjectById(t){return this.getObjectByProperty("id",t)}getObjectByName(t){return this.getObjectByProperty("name",t)}getObjectByProperty(t,e){if(this[t]===e)return this;for(let i=0,s=this.children.length;i<s;i++){let o=this.children[i].getObjectByProperty(t,e);if(o!==void 0)return o}}getObjectsByProperty(t,e,i=[]){this[t]===e&&i.push(this);let s=this.children;for(let r=0,o=s.length;r<o;r++)s[r].getObjectsByProperty(t,e,i);return i}getWorldPosition(t){return this.updateWorldMatrix(!0,!1),t.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(t){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose($s,t,Hp),t}getWorldScale(t){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose($s,Vp,t),t}getWorldDirection(t){this.updateWorldMatrix(!0,!1);let e=this.matrixWorld.elements;return t.set(e[8],e[9],e[10]).normalize()}raycast(){}traverse(t){t(this);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].traverse(t)}traverseVisible(t){if(this.visible===!1)return;t(this);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].traverseVisible(t)}traverseAncestors(t){let e=this.parent;e!==null&&(t(e),e.traverseAncestors(t))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale),this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(t){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||t)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,t=!0);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].updateMatrixWorld(t)}updateWorldMatrix(t,e){let i=this.parent;if(t===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),e===!0){let s=this.children;for(let r=0,o=s.length;r<o;r++)s[r].updateWorldMatrix(!1,!0)}}toJSON(t){let e=t===void 0||typeof t=="string",i={};e&&(t={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},i.metadata={version:4.6,type:"Object",generator:"Object3D.toJSON"});let s={};s.uuid=this.uuid,s.type=this.type,this.name!==""&&(s.name=this.name),this.castShadow===!0&&(s.castShadow=!0),this.receiveShadow===!0&&(s.receiveShadow=!0),this.visible===!1&&(s.visible=!1),this.frustumCulled===!1&&(s.frustumCulled=!1),this.renderOrder!==0&&(s.renderOrder=this.renderOrder),Object.keys(this.userData).length>0&&(s.userData=this.userData),s.layers=this.layers.mask,s.matrix=this.matrix.toArray(),s.up=this.up.toArray(),this.matrixAutoUpdate===!1&&(s.matrixAutoUpdate=!1),this.isInstancedMesh&&(s.type="InstancedMesh",s.count=this.count,s.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(s.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(s.type="BatchedMesh",s.perObjectFrustumCulled=this.perObjectFrustumCulled,s.sortObjects=this.sortObjects,s.drawRanges=this._drawRanges,s.reservedRanges=this._reservedRanges,s.visibility=this._visibility,s.active=this._active,s.bounds=this._bounds.map(a=>({boxInitialized:a.boxInitialized,boxMin:a.box.min.toArray(),boxMax:a.box.max.toArray(),sphereInitialized:a.sphereInitialized,sphereRadius:a.sphere.radius,sphereCenter:a.sphere.center.toArray()})),s.maxInstanceCount=this._maxInstanceCount,s.maxVertexCount=this._maxVertexCount,s.maxIndexCount=this._maxIndexCount,s.geometryInitialized=this._geometryInitialized,s.geometryCount=this._geometryCount,s.matricesTexture=this._matricesTexture.toJSON(t),this._colorsTexture!==null&&(s.colorsTexture=this._colorsTexture.toJSON(t)),this.boundingSphere!==null&&(s.boundingSphere={center:s.boundingSphere.center.toArray(),radius:s.boundingSphere.radius}),this.boundingBox!==null&&(s.boundingBox={min:s.boundingBox.min.toArray(),max:s.boundingBox.max.toArray()}));function r(a,c){return a[c.uuid]===void 0&&(a[c.uuid]=c.toJSON(t)),c.uuid}if(this.isScene)this.background&&(this.background.isColor?s.background=this.background.toJSON():this.background.isTexture&&(s.background=this.background.toJSON(t).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(s.environment=this.environment.toJSON(t).uuid);else if(this.isMesh||this.isLine||this.isPoints){s.geometry=r(t.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let c=a.shapes;if(Array.isArray(c))for(let l=0,f=c.length;l<f;l++){let d=c[l];r(t.shapes,d)}else r(t.shapes,c)}}if(this.isSkinnedMesh&&(s.bindMode=this.bindMode,s.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(t.skeletons,this.skeleton),s.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let c=0,l=this.material.length;c<l;c++)a.push(r(t.materials,this.material[c]));s.material=a}else s.material=r(t.materials,this.material);if(this.children.length>0){s.children=[];for(let a=0;a<this.children.length;a++)s.children.push(this.children[a].toJSON(t).object)}if(this.animations.length>0){s.animations=[];for(let a=0;a<this.animations.length;a++){let c=this.animations[a];s.animations.push(r(t.animations,c))}}if(e){let a=o(t.geometries),c=o(t.materials),l=o(t.textures),f=o(t.images),d=o(t.shapes),h=o(t.skeletons),m=o(t.animations),x=o(t.nodes);a.length>0&&(i.geometries=a),c.length>0&&(i.materials=c),l.length>0&&(i.textures=l),f.length>0&&(i.images=f),d.length>0&&(i.shapes=d),h.length>0&&(i.skeletons=h),m.length>0&&(i.animations=m),x.length>0&&(i.nodes=x)}return i.object=s,i;function o(a){let c=[];for(let l in a){let f=a[l];delete f.metadata,c.push(f)}return c}}clone(t){return new this.constructor().copy(this,t)}copy(t,e=!0){if(this.name=t.name,this.up.copy(t.up),this.position.copy(t.position),this.rotation.order=t.rotation.order,this.quaternion.copy(t.quaternion),this.scale.copy(t.scale),this.matrix.copy(t.matrix),this.matrixWorld.copy(t.matrixWorld),this.matrixAutoUpdate=t.matrixAutoUpdate,this.matrixWorldAutoUpdate=t.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=t.matrixWorldNeedsUpdate,this.layers.mask=t.layers.mask,this.visible=t.visible,this.castShadow=t.castShadow,this.receiveShadow=t.receiveShadow,this.frustumCulled=t.frustumCulled,this.renderOrder=t.renderOrder,this.animations=t.animations.slice(),this.userData=JSON.parse(JSON.stringify(t.userData)),e===!0)for(let i=0;i<t.children.length;i++){let s=t.children[i];this.add(s.clone())}return this}};He.DEFAULT_UP=new D(0,1,0);He.DEFAULT_MATRIX_AUTO_UPDATE=!0;He.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var un=new D,Ln=new D,$a=new D,Dn=new D,ss=new D,rs=new D,su=new D,tc=new D,ec=new D,nc=new D,ic=new ce,sc=new ce,rc=new ce,Si=class n{constructor(t=new D,e=new D,i=new D){this.a=t,this.b=e,this.c=i}static getNormal(t,e,i,s){s.subVectors(i,e),un.subVectors(t,e),s.cross(un);let r=s.lengthSq();return r>0?s.multiplyScalar(1/Math.sqrt(r)):s.set(0,0,0)}static getBarycoord(t,e,i,s,r){un.subVectors(s,e),Ln.subVectors(i,e),$a.subVectors(t,e);let o=un.dot(un),a=un.dot(Ln),c=un.dot($a),l=Ln.dot(Ln),f=Ln.dot($a),d=o*l-a*a;if(d===0)return r.set(0,0,0),null;let h=1/d,m=(l*c-a*f)*h,x=(o*f-a*c)*h;return r.set(1-m-x,x,m)}static containsPoint(t,e,i,s){return this.getBarycoord(t,e,i,s,Dn)===null?!1:Dn.x>=0&&Dn.y>=0&&Dn.x+Dn.y<=1}static getInterpolation(t,e,i,s,r,o,a,c){return this.getBarycoord(t,e,i,s,Dn)===null?(c.x=0,c.y=0,"z"in c&&(c.z=0),"w"in c&&(c.w=0),null):(c.setScalar(0),c.addScaledVector(r,Dn.x),c.addScaledVector(o,Dn.y),c.addScaledVector(a,Dn.z),c)}static getInterpolatedAttribute(t,e,i,s,r,o){return ic.setScalar(0),sc.setScalar(0),rc.setScalar(0),ic.fromBufferAttribute(t,e),sc.fromBufferAttribute(t,i),rc.fromBufferAttribute(t,s),o.setScalar(0),o.addScaledVector(ic,r.x),o.addScaledVector(sc,r.y),o.addScaledVector(rc,r.z),o}static isFrontFacing(t,e,i,s){return un.subVectors(i,e),Ln.subVectors(t,e),un.cross(Ln).dot(s)<0}set(t,e,i){return this.a.copy(t),this.b.copy(e),this.c.copy(i),this}setFromPointsAndIndices(t,e,i,s){return this.a.copy(t[e]),this.b.copy(t[i]),this.c.copy(t[s]),this}setFromAttributeAndIndices(t,e,i,s){return this.a.fromBufferAttribute(t,e),this.b.fromBufferAttribute(t,i),this.c.fromBufferAttribute(t,s),this}clone(){return new this.constructor().copy(this)}copy(t){return this.a.copy(t.a),this.b.copy(t.b),this.c.copy(t.c),this}getArea(){return un.subVectors(this.c,this.b),Ln.subVectors(this.a,this.b),un.cross(Ln).length()*.5}getMidpoint(t){return t.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(t){return n.getNormal(this.a,this.b,this.c,t)}getPlane(t){return t.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(t,e){return n.getBarycoord(t,this.a,this.b,this.c,e)}getInterpolation(t,e,i,s,r){return n.getInterpolation(t,this.a,this.b,this.c,e,i,s,r)}containsPoint(t){return n.containsPoint(t,this.a,this.b,this.c)}isFrontFacing(t){return n.isFrontFacing(this.a,this.b,this.c,t)}intersectsBox(t){return t.intersectsTriangle(this)}closestPointToPoint(t,e){let i=this.a,s=this.b,r=this.c,o,a;ss.subVectors(s,i),rs.subVectors(r,i),tc.subVectors(t,i);let c=ss.dot(tc),l=rs.dot(tc);if(c<=0&&l<=0)return e.copy(i);ec.subVectors(t,s);let f=ss.dot(ec),d=rs.dot(ec);if(f>=0&&d<=f)return e.copy(s);let h=c*d-f*l;if(h<=0&&c>=0&&f<=0)return o=c/(c-f),e.copy(i).addScaledVector(ss,o);nc.subVectors(t,r);let m=ss.dot(nc),x=rs.dot(nc);if(x>=0&&m<=x)return e.copy(r);let _=m*l-c*x;if(_<=0&&l>=0&&x<=0)return a=l/(l-x),e.copy(i).addScaledVector(rs,a);let u=f*x-m*d;if(u<=0&&d-f>=0&&m-x>=0)return su.subVectors(r,s),a=(d-f)/(d-f+(m-x)),e.copy(s).addScaledVector(su,a);let p=1/(u+_+h);return o=_*p,a=h*p,e.copy(i).addScaledVector(ss,o).addScaledVector(rs,a)}equals(t){return t.a.equals(this.a)&&t.b.equals(this.b)&&t.c.equals(this.c)}},ed={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Kn={h:0,s:0,l:0},qr={h:0,s:0,l:0};function oc(n,t,e){return e<0&&(e+=1),e>1&&(e-=1),e<1/6?n+(t-n)*6*e:e<1/2?t:e<2/3?n+(t-n)*6*(2/3-e):n}var Ot=class{constructor(t,e,i){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(t,e,i)}set(t,e,i){if(e===void 0&&i===void 0){let s=t;s&&s.isColor?this.copy(s):typeof s=="number"?this.setHex(s):typeof s=="string"&&this.setStyle(s)}else this.setRGB(t,e,i);return this}setScalar(t){return this.r=t,this.g=t,this.b=t,this}setHex(t,e=yn){return t=Math.floor(t),this.r=(t>>16&255)/255,this.g=(t>>8&255)/255,this.b=(t&255)/255,Jt.toWorkingColorSpace(this,e),this}setRGB(t,e,i,s=Jt.workingColorSpace){return this.r=t,this.g=e,this.b=i,Jt.toWorkingColorSpace(this,s),this}setHSL(t,e,i,s=Jt.workingColorSpace){if(t=Vl(t,1),e=Ue(e,0,1),i=Ue(i,0,1),e===0)this.r=this.g=this.b=i;else{let r=i<=.5?i*(1+e):i+e-i*e,o=2*i-r;this.r=oc(o,r,t+1/3),this.g=oc(o,r,t),this.b=oc(o,r,t-1/3)}return Jt.toWorkingColorSpace(this,s),this}setStyle(t,e=yn){function i(r){r!==void 0&&parseFloat(r)<1&&console.warn("THREE.Color: Alpha component of "+t+" will be ignored.")}let s;if(s=/^(\w+)\(([^\)]*)\)/.exec(t)){let r,o=s[1],a=s[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,e);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,e);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,e);break;default:console.warn("THREE.Color: Unknown color model "+t)}}else if(s=/^\#([A-Fa-f\d]+)$/.exec(t)){let r=s[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,e);if(o===6)return this.setHex(parseInt(r,16),e);console.warn("THREE.Color: Invalid hex color "+t)}else if(t&&t.length>0)return this.setColorName(t,e);return this}setColorName(t,e=yn){let i=ed[t.toLowerCase()];return i!==void 0?this.setHex(i,e):console.warn("THREE.Color: Unknown color "+t),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(t){return this.r=t.r,this.g=t.g,this.b=t.b,this}copySRGBToLinear(t){return this.r=ms(t.r),this.g=ms(t.g),this.b=ms(t.b),this}copyLinearToSRGB(t){return this.r=Wa(t.r),this.g=Wa(t.g),this.b=Wa(t.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(t=yn){return Jt.fromWorkingColorSpace(Re.copy(this),t),Math.round(Ue(Re.r*255,0,255))*65536+Math.round(Ue(Re.g*255,0,255))*256+Math.round(Ue(Re.b*255,0,255))}getHexString(t=yn){return("000000"+this.getHex(t).toString(16)).slice(-6)}getHSL(t,e=Jt.workingColorSpace){Jt.fromWorkingColorSpace(Re.copy(this),e);let i=Re.r,s=Re.g,r=Re.b,o=Math.max(i,s,r),a=Math.min(i,s,r),c,l,f=(a+o)/2;if(a===o)c=0,l=0;else{let d=o-a;switch(l=f<=.5?d/(o+a):d/(2-o-a),o){case i:c=(s-r)/d+(s<r?6:0);break;case s:c=(r-i)/d+2;break;case r:c=(i-s)/d+4;break}c/=6}return t.h=c,t.s=l,t.l=f,t}getRGB(t,e=Jt.workingColorSpace){return Jt.fromWorkingColorSpace(Re.copy(this),e),t.r=Re.r,t.g=Re.g,t.b=Re.b,t}getStyle(t=yn){Jt.fromWorkingColorSpace(Re.copy(this),t);let e=Re.r,i=Re.g,s=Re.b;return t!==yn?`color(${t} ${e.toFixed(3)} ${i.toFixed(3)} ${s.toFixed(3)})`:`rgb(${Math.round(e*255)},${Math.round(i*255)},${Math.round(s*255)})`}offsetHSL(t,e,i){return this.getHSL(Kn),this.setHSL(Kn.h+t,Kn.s+e,Kn.l+i)}add(t){return this.r+=t.r,this.g+=t.g,this.b+=t.b,this}addColors(t,e){return this.r=t.r+e.r,this.g=t.g+e.g,this.b=t.b+e.b,this}addScalar(t){return this.r+=t,this.g+=t,this.b+=t,this}sub(t){return this.r=Math.max(0,this.r-t.r),this.g=Math.max(0,this.g-t.g),this.b=Math.max(0,this.b-t.b),this}multiply(t){return this.r*=t.r,this.g*=t.g,this.b*=t.b,this}multiplyScalar(t){return this.r*=t,this.g*=t,this.b*=t,this}lerp(t,e){return this.r+=(t.r-this.r)*e,this.g+=(t.g-this.g)*e,this.b+=(t.b-this.b)*e,this}lerpColors(t,e,i){return this.r=t.r+(e.r-t.r)*i,this.g=t.g+(e.g-t.g)*i,this.b=t.b+(e.b-t.b)*i,this}lerpHSL(t,e){this.getHSL(Kn),t.getHSL(qr);let i=rr(Kn.h,qr.h,e),s=rr(Kn.s,qr.s,e),r=rr(Kn.l,qr.l,e);return this.setHSL(i,s,r),this}setFromVector3(t){return this.r=t.x,this.g=t.y,this.b=t.z,this}applyMatrix3(t){let e=this.r,i=this.g,s=this.b,r=t.elements;return this.r=r[0]*e+r[3]*i+r[6]*s,this.g=r[1]*e+r[4]*i+r[7]*s,this.b=r[2]*e+r[5]*i+r[8]*s,this}equals(t){return t.r===this.r&&t.g===this.g&&t.b===this.b}fromArray(t,e=0){return this.r=t[e],this.g=t[e+1],this.b=t[e+2],this}toArray(t=[],e=0){return t[e]=this.r,t[e+1]=this.g,t[e+2]=this.b,t}fromBufferAttribute(t,e){return this.r=t.getX(e),this.g=t.getY(e),this.b=t.getZ(e),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},Re=new Ot;Ot.NAMES=ed;var Wp=0,Ri=class extends Bn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Wp++}),this.uuid=Es(),this.name="",this.type="Material",this.blending=fs,this.side=ei,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=xc,this.blendDst=vc,this.blendEquation=bi,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Ot(0,0,0),this.blendAlpha=0,this.depthFunc=xs,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=Gh,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=Ki,this.stencilZFail=Ki,this.stencilZPass=Ki,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(t){this._alphaTest>0!=t>0&&this.version++,this._alphaTest=t}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(t){if(t!==void 0)for(let e in t){let i=t[e];if(i===void 0){console.warn(`THREE.Material: parameter '${e}' has value of undefined.`);continue}let s=this[e];if(s===void 0){console.warn(`THREE.Material: '${e}' is not a property of THREE.${this.type}.`);continue}s&&s.isColor?s.set(i):s&&s.isVector3&&i&&i.isVector3?s.copy(i):this[e]=i}}toJSON(t){let e=t===void 0||typeof t=="string";e&&(t={textures:{},images:{}});let i={metadata:{version:4.6,type:"Material",generator:"Material.toJSON"}};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.color&&this.color.isColor&&(i.color=this.color.getHex()),this.roughness!==void 0&&(i.roughness=this.roughness),this.metalness!==void 0&&(i.metalness=this.metalness),this.sheen!==void 0&&(i.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(i.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(i.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(i.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(i.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(i.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(i.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(i.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(i.shininess=this.shininess),this.clearcoat!==void 0&&(i.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(i.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(i.clearcoatMap=this.clearcoatMap.toJSON(t).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(i.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(t).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(i.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(t).uuid,i.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.dispersion!==void 0&&(i.dispersion=this.dispersion),this.iridescence!==void 0&&(i.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(i.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(i.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(i.iridescenceMap=this.iridescenceMap.toJSON(t).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(i.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(t).uuid),this.anisotropy!==void 0&&(i.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(i.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(i.anisotropyMap=this.anisotropyMap.toJSON(t).uuid),this.map&&this.map.isTexture&&(i.map=this.map.toJSON(t).uuid),this.matcap&&this.matcap.isTexture&&(i.matcap=this.matcap.toJSON(t).uuid),this.alphaMap&&this.alphaMap.isTexture&&(i.alphaMap=this.alphaMap.toJSON(t).uuid),this.lightMap&&this.lightMap.isTexture&&(i.lightMap=this.lightMap.toJSON(t).uuid,i.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(i.aoMap=this.aoMap.toJSON(t).uuid,i.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(i.bumpMap=this.bumpMap.toJSON(t).uuid,i.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(i.normalMap=this.normalMap.toJSON(t).uuid,i.normalMapType=this.normalMapType,i.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(i.displacementMap=this.displacementMap.toJSON(t).uuid,i.displacementScale=this.displacementScale,i.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(i.roughnessMap=this.roughnessMap.toJSON(t).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(i.metalnessMap=this.metalnessMap.toJSON(t).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(i.emissiveMap=this.emissiveMap.toJSON(t).uuid),this.specularMap&&this.specularMap.isTexture&&(i.specularMap=this.specularMap.toJSON(t).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(i.specularIntensityMap=this.specularIntensityMap.toJSON(t).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(i.specularColorMap=this.specularColorMap.toJSON(t).uuid),this.envMap&&this.envMap.isTexture&&(i.envMap=this.envMap.toJSON(t).uuid,this.combine!==void 0&&(i.combine=this.combine)),this.envMapRotation!==void 0&&(i.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(i.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(i.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(i.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(i.gradientMap=this.gradientMap.toJSON(t).uuid),this.transmission!==void 0&&(i.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(i.transmissionMap=this.transmissionMap.toJSON(t).uuid),this.thickness!==void 0&&(i.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(i.thicknessMap=this.thicknessMap.toJSON(t).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(i.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(i.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(i.size=this.size),this.shadowSide!==null&&(i.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(i.sizeAttenuation=this.sizeAttenuation),this.blending!==fs&&(i.blending=this.blending),this.side!==ei&&(i.side=this.side),this.vertexColors===!0&&(i.vertexColors=!0),this.opacity<1&&(i.opacity=this.opacity),this.transparent===!0&&(i.transparent=!0),this.blendSrc!==xc&&(i.blendSrc=this.blendSrc),this.blendDst!==vc&&(i.blendDst=this.blendDst),this.blendEquation!==bi&&(i.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(i.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(i.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(i.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(i.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(i.blendAlpha=this.blendAlpha),this.depthFunc!==xs&&(i.depthFunc=this.depthFunc),this.depthTest===!1&&(i.depthTest=this.depthTest),this.depthWrite===!1&&(i.depthWrite=this.depthWrite),this.colorWrite===!1&&(i.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(i.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==Gh&&(i.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(i.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(i.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==Ki&&(i.stencilFail=this.stencilFail),this.stencilZFail!==Ki&&(i.stencilZFail=this.stencilZFail),this.stencilZPass!==Ki&&(i.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(i.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(i.rotation=this.rotation),this.polygonOffset===!0&&(i.polygonOffset=!0),this.polygonOffsetFactor!==0&&(i.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(i.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(i.linewidth=this.linewidth),this.dashSize!==void 0&&(i.dashSize=this.dashSize),this.gapSize!==void 0&&(i.gapSize=this.gapSize),this.scale!==void 0&&(i.scale=this.scale),this.dithering===!0&&(i.dithering=!0),this.alphaTest>0&&(i.alphaTest=this.alphaTest),this.alphaHash===!0&&(i.alphaHash=!0),this.alphaToCoverage===!0&&(i.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(i.premultipliedAlpha=!0),this.forceSinglePass===!0&&(i.forceSinglePass=!0),this.wireframe===!0&&(i.wireframe=!0),this.wireframeLinewidth>1&&(i.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(i.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(i.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(i.flatShading=!0),this.visible===!1&&(i.visible=!1),this.toneMapped===!1&&(i.toneMapped=!1),this.fog===!1&&(i.fog=!1),Object.keys(this.userData).length>0&&(i.userData=this.userData);function s(r){let o=[];for(let a in r){let c=r[a];delete c.metadata,o.push(c)}return o}if(e){let r=s(t.textures),o=s(t.images);r.length>0&&(i.textures=r),o.length>0&&(i.images=o)}return i}clone(){return new this.constructor().copy(this)}copy(t){this.name=t.name,this.blending=t.blending,this.side=t.side,this.vertexColors=t.vertexColors,this.opacity=t.opacity,this.transparent=t.transparent,this.blendSrc=t.blendSrc,this.blendDst=t.blendDst,this.blendEquation=t.blendEquation,this.blendSrcAlpha=t.blendSrcAlpha,this.blendDstAlpha=t.blendDstAlpha,this.blendEquationAlpha=t.blendEquationAlpha,this.blendColor.copy(t.blendColor),this.blendAlpha=t.blendAlpha,this.depthFunc=t.depthFunc,this.depthTest=t.depthTest,this.depthWrite=t.depthWrite,this.stencilWriteMask=t.stencilWriteMask,this.stencilFunc=t.stencilFunc,this.stencilRef=t.stencilRef,this.stencilFuncMask=t.stencilFuncMask,this.stencilFail=t.stencilFail,this.stencilZFail=t.stencilZFail,this.stencilZPass=t.stencilZPass,this.stencilWrite=t.stencilWrite;let e=t.clippingPlanes,i=null;if(e!==null){let s=e.length;i=new Array(s);for(let r=0;r!==s;++r)i[r]=e[r].clone()}return this.clippingPlanes=i,this.clipIntersection=t.clipIntersection,this.clipShadows=t.clipShadows,this.shadowSide=t.shadowSide,this.colorWrite=t.colorWrite,this.precision=t.precision,this.polygonOffset=t.polygonOffset,this.polygonOffsetFactor=t.polygonOffsetFactor,this.polygonOffsetUnits=t.polygonOffsetUnits,this.dithering=t.dithering,this.alphaTest=t.alphaTest,this.alphaHash=t.alphaHash,this.alphaToCoverage=t.alphaToCoverage,this.premultipliedAlpha=t.premultipliedAlpha,this.forceSinglePass=t.forceSinglePass,this.visible=t.visible,this.toneMapped=t.toneMapped,this.userData=JSON.parse(JSON.stringify(t.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(t){t===!0&&this.version++}onBuild(){console.warn("Material: onBuild() has been removed.")}},bs=class extends Ri{constructor(t){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Ot(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new nn,this.combine=Hu,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(t)}copy(t){return super.copy(t),this.color.copy(t.color),this.map=t.map,this.lightMap=t.lightMap,this.lightMapIntensity=t.lightMapIntensity,this.aoMap=t.aoMap,this.aoMapIntensity=t.aoMapIntensity,this.specularMap=t.specularMap,this.alphaMap=t.alphaMap,this.envMap=t.envMap,this.envMapRotation.copy(t.envMapRotation),this.combine=t.combine,this.reflectivity=t.reflectivity,this.refractionRatio=t.refractionRatio,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.wireframeLinecap=t.wireframeLinecap,this.wireframeLinejoin=t.wireframeLinejoin,this.fog=t.fog,this}};var de=new D,Yr=new yt,Ke=class{constructor(t,e,i=!1){if(Array.isArray(t))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,this.name="",this.array=t,this.itemSize=e,this.count=t!==void 0?t.length/e:0,this.normalized=i,this.usage=Wh,this.updateRanges=[],this.gpuType=bn,this.version=0}onUploadCallback(){}set needsUpdate(t){t===!0&&this.version++}setUsage(t){return this.usage=t,this}addUpdateRange(t,e){this.updateRanges.push({start:t,count:e})}clearUpdateRanges(){this.updateRanges.length=0}copy(t){return this.name=t.name,this.array=new t.array.constructor(t.array),this.itemSize=t.itemSize,this.count=t.count,this.normalized=t.normalized,this.usage=t.usage,this.gpuType=t.gpuType,this}copyAt(t,e,i){t*=this.itemSize,i*=e.itemSize;for(let s=0,r=this.itemSize;s<r;s++)this.array[t+s]=e.array[i+s];return this}copyArray(t){return this.array.set(t),this}applyMatrix3(t){if(this.itemSize===2)for(let e=0,i=this.count;e<i;e++)Yr.fromBufferAttribute(this,e),Yr.applyMatrix3(t),this.setXY(e,Yr.x,Yr.y);else if(this.itemSize===3)for(let e=0,i=this.count;e<i;e++)de.fromBufferAttribute(this,e),de.applyMatrix3(t),this.setXYZ(e,de.x,de.y,de.z);return this}applyMatrix4(t){for(let e=0,i=this.count;e<i;e++)de.fromBufferAttribute(this,e),de.applyMatrix4(t),this.setXYZ(e,de.x,de.y,de.z);return this}applyNormalMatrix(t){for(let e=0,i=this.count;e<i;e++)de.fromBufferAttribute(this,e),de.applyNormalMatrix(t),this.setXYZ(e,de.x,de.y,de.z);return this}transformDirection(t){for(let e=0,i=this.count;e<i;e++)de.fromBufferAttribute(this,e),de.transformDirection(t),this.setXYZ(e,de.x,de.y,de.z);return this}set(t,e=0){return this.array.set(t,e),this}getComponent(t,e){let i=this.array[t*this.itemSize+e];return this.normalized&&(i=us(i,this.array)),i}setComponent(t,e,i){return this.normalized&&(i=Le(i,this.array)),this.array[t*this.itemSize+e]=i,this}getX(t){let e=this.array[t*this.itemSize];return this.normalized&&(e=us(e,this.array)),e}setX(t,e){return this.normalized&&(e=Le(e,this.array)),this.array[t*this.itemSize]=e,this}getY(t){let e=this.array[t*this.itemSize+1];return this.normalized&&(e=us(e,this.array)),e}setY(t,e){return this.normalized&&(e=Le(e,this.array)),this.array[t*this.itemSize+1]=e,this}getZ(t){let e=this.array[t*this.itemSize+2];return this.normalized&&(e=us(e,this.array)),e}setZ(t,e){return this.normalized&&(e=Le(e,this.array)),this.array[t*this.itemSize+2]=e,this}getW(t){let e=this.array[t*this.itemSize+3];return this.normalized&&(e=us(e,this.array)),e}setW(t,e){return this.normalized&&(e=Le(e,this.array)),this.array[t*this.itemSize+3]=e,this}setXY(t,e,i){return t*=this.itemSize,this.normalized&&(e=Le(e,this.array),i=Le(i,this.array)),this.array[t+0]=e,this.array[t+1]=i,this}setXYZ(t,e,i,s){return t*=this.itemSize,this.normalized&&(e=Le(e,this.array),i=Le(i,this.array),s=Le(s,this.array)),this.array[t+0]=e,this.array[t+1]=i,this.array[t+2]=s,this}setXYZW(t,e,i,s,r){return t*=this.itemSize,this.normalized&&(e=Le(e,this.array),i=Le(i,this.array),s=Le(s,this.array),r=Le(r,this.array)),this.array[t+0]=e,this.array[t+1]=i,this.array[t+2]=s,this.array[t+3]=r,this}onUpload(t){return this.onUploadCallback=t,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let t={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(t.name=this.name),this.usage!==Wh&&(t.usage=this.usage),t}};var So=class extends Ke{constructor(t,e,i){super(new Uint16Array(t),e,i)}};var wo=class extends Ke{constructor(t,e,i){super(new Uint32Array(t),e,i)}};var be=class extends Ke{constructor(t,e,i){super(new Float32Array(t),e,i)}},Xp=0,en=new $t,ac=new He,os=new D,Ze=new zn,tr=new zn,Me=new D,pn=class n extends Bn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:Xp++}),this.uuid=Es(),this.name="",this.type="BufferGeometry",this.index=null,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={}}getIndex(){return this.index}setIndex(t){return Array.isArray(t)?this.index=new(td(t)?wo:So)(t,1):this.index=t,this}getAttribute(t){return this.attributes[t]}setAttribute(t,e){return this.attributes[t]=e,this}deleteAttribute(t){return delete this.attributes[t],this}hasAttribute(t){return this.attributes[t]!==void 0}addGroup(t,e,i=0){this.groups.push({start:t,count:e,materialIndex:i})}clearGroups(){this.groups=[]}setDrawRange(t,e){this.drawRange.start=t,this.drawRange.count=e}applyMatrix4(t){let e=this.attributes.position;e!==void 0&&(e.applyMatrix4(t),e.needsUpdate=!0);let i=this.attributes.normal;if(i!==void 0){let r=new zt().getNormalMatrix(t);i.applyNormalMatrix(r),i.needsUpdate=!0}let s=this.attributes.tangent;return s!==void 0&&(s.transformDirection(t),s.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this}applyQuaternion(t){return en.makeRotationFromQuaternion(t),this.applyMatrix4(en),this}rotateX(t){return en.makeRotationX(t),this.applyMatrix4(en),this}rotateY(t){return en.makeRotationY(t),this.applyMatrix4(en),this}rotateZ(t){return en.makeRotationZ(t),this.applyMatrix4(en),this}translate(t,e,i){return en.makeTranslation(t,e,i),this.applyMatrix4(en),this}scale(t,e,i){return en.makeScale(t,e,i),this.applyMatrix4(en),this}lookAt(t){return ac.lookAt(t),ac.updateMatrix(),this.applyMatrix4(ac.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(os).negate(),this.translate(os.x,os.y,os.z),this}setFromPoints(t){let e=[];for(let i=0,s=t.length;i<s;i++){let r=t[i];e.push(r.x,r.y,r.z||0)}return this.setAttribute("position",new be(e,3)),this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new zn);let t=this.attributes.position,e=this.morphAttributes.position;if(t&&t.isGLBufferAttribute){console.error("THREE.BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new D(-1/0,-1/0,-1/0),new D(1/0,1/0,1/0));return}if(t!==void 0){if(this.boundingBox.setFromBufferAttribute(t),e)for(let i=0,s=e.length;i<s;i++){let r=e[i];Ze.setFromBufferAttribute(r),this.morphTargetsRelative?(Me.addVectors(this.boundingBox.min,Ze.min),this.boundingBox.expandByPoint(Me),Me.addVectors(this.boundingBox.max,Ze.max),this.boundingBox.expandByPoint(Me)):(this.boundingBox.expandByPoint(Ze.min),this.boundingBox.expandByPoint(Ze.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&console.error('THREE.BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new Ci);let t=this.attributes.position,e=this.morphAttributes.position;if(t&&t.isGLBufferAttribute){console.error("THREE.BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new D,1/0);return}if(t){let i=this.boundingSphere.center;if(Ze.setFromBufferAttribute(t),e)for(let r=0,o=e.length;r<o;r++){let a=e[r];tr.setFromBufferAttribute(a),this.morphTargetsRelative?(Me.addVectors(Ze.min,tr.min),Ze.expandByPoint(Me),Me.addVectors(Ze.max,tr.max),Ze.expandByPoint(Me)):(Ze.expandByPoint(tr.min),Ze.expandByPoint(tr.max))}Ze.getCenter(i);let s=0;for(let r=0,o=t.count;r<o;r++)Me.fromBufferAttribute(t,r),s=Math.max(s,i.distanceToSquared(Me));if(e)for(let r=0,o=e.length;r<o;r++){let a=e[r],c=this.morphTargetsRelative;for(let l=0,f=a.count;l<f;l++)Me.fromBufferAttribute(a,l),c&&(os.fromBufferAttribute(t,l),Me.add(os)),s=Math.max(s,i.distanceToSquared(Me))}this.boundingSphere.radius=Math.sqrt(s),isNaN(this.boundingSphere.radius)&&console.error('THREE.BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let t=this.index,e=this.attributes;if(t===null||e.position===void 0||e.normal===void 0||e.uv===void 0){console.error("THREE.BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let i=e.position,s=e.normal,r=e.uv;this.hasAttribute("tangent")===!1&&this.setAttribute("tangent",new Ke(new Float32Array(4*i.count),4));let o=this.getAttribute("tangent"),a=[],c=[];for(let C=0;C<i.count;C++)a[C]=new D,c[C]=new D;let l=new D,f=new D,d=new D,h=new yt,m=new yt,x=new yt,_=new D,u=new D;function p(C,N,g){l.fromBufferAttribute(i,C),f.fromBufferAttribute(i,N),d.fromBufferAttribute(i,g),h.fromBufferAttribute(r,C),m.fromBufferAttribute(r,N),x.fromBufferAttribute(r,g),f.sub(l),d.sub(l),m.sub(h),x.sub(h);let S=1/(m.x*x.y-x.x*m.y);isFinite(S)&&(_.copy(f).multiplyScalar(x.y).addScaledVector(d,-m.y).multiplyScalar(S),u.copy(d).multiplyScalar(m.x).addScaledVector(f,-x.x).multiplyScalar(S),a[C].add(_),a[N].add(_),a[g].add(_),c[C].add(u),c[N].add(u),c[g].add(u))}let b=this.groups;b.length===0&&(b=[{start:0,count:t.count}]);for(let C=0,N=b.length;C<N;++C){let g=b[C],S=g.start,F=g.count;for(let V=S,q=S+F;V<q;V+=3)p(t.getX(V+0),t.getX(V+1),t.getX(V+2))}let y=new D,A=new D,R=new D,T=new D;function E(C){R.fromBufferAttribute(s,C),T.copy(R);let N=a[C];y.copy(N),y.sub(R.multiplyScalar(R.dot(N))).normalize(),A.crossVectors(T,N);let S=A.dot(c[C])<0?-1:1;o.setXYZW(C,y.x,y.y,y.z,S)}for(let C=0,N=b.length;C<N;++C){let g=b[C],S=g.start,F=g.count;for(let V=S,q=S+F;V<q;V+=3)E(t.getX(V+0)),E(t.getX(V+1)),E(t.getX(V+2))}}computeVertexNormals(){let t=this.index,e=this.getAttribute("position");if(e!==void 0){let i=this.getAttribute("normal");if(i===void 0)i=new Ke(new Float32Array(e.count*3),3),this.setAttribute("normal",i);else for(let h=0,m=i.count;h<m;h++)i.setXYZ(h,0,0,0);let s=new D,r=new D,o=new D,a=new D,c=new D,l=new D,f=new D,d=new D;if(t)for(let h=0,m=t.count;h<m;h+=3){let x=t.getX(h+0),_=t.getX(h+1),u=t.getX(h+2);s.fromBufferAttribute(e,x),r.fromBufferAttribute(e,_),o.fromBufferAttribute(e,u),f.subVectors(o,r),d.subVectors(s,r),f.cross(d),a.fromBufferAttribute(i,x),c.fromBufferAttribute(i,_),l.fromBufferAttribute(i,u),a.add(f),c.add(f),l.add(f),i.setXYZ(x,a.x,a.y,a.z),i.setXYZ(_,c.x,c.y,c.z),i.setXYZ(u,l.x,l.y,l.z)}else for(let h=0,m=e.count;h<m;h+=3)s.fromBufferAttribute(e,h+0),r.fromBufferAttribute(e,h+1),o.fromBufferAttribute(e,h+2),f.subVectors(o,r),d.subVectors(s,r),f.cross(d),i.setXYZ(h+0,f.x,f.y,f.z),i.setXYZ(h+1,f.x,f.y,f.z),i.setXYZ(h+2,f.x,f.y,f.z);this.normalizeNormals(),i.needsUpdate=!0}}normalizeNormals(){let t=this.attributes.normal;for(let e=0,i=t.count;e<i;e++)Me.fromBufferAttribute(t,e),Me.normalize(),t.setXYZ(e,Me.x,Me.y,Me.z)}toNonIndexed(){function t(a,c){let l=a.array,f=a.itemSize,d=a.normalized,h=new l.constructor(c.length*f),m=0,x=0;for(let _=0,u=c.length;_<u;_++){a.isInterleavedBufferAttribute?m=c[_]*a.data.stride+a.offset:m=c[_]*f;for(let p=0;p<f;p++)h[x++]=l[m++]}return new Ke(h,f,d)}if(this.index===null)return console.warn("THREE.BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let e=new n,i=this.index.array,s=this.attributes;for(let a in s){let c=s[a],l=t(c,i);e.setAttribute(a,l)}let r=this.morphAttributes;for(let a in r){let c=[],l=r[a];for(let f=0,d=l.length;f<d;f++){let h=l[f],m=t(h,i);c.push(m)}e.morphAttributes[a]=c}e.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,c=o.length;a<c;a++){let l=o[a];e.addGroup(l.start,l.count,l.materialIndex)}return e}toJSON(){let t={metadata:{version:4.6,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(t.uuid=this.uuid,t.type=this.type,this.name!==""&&(t.name=this.name),Object.keys(this.userData).length>0&&(t.userData=this.userData),this.parameters!==void 0){let c=this.parameters;for(let l in c)c[l]!==void 0&&(t[l]=c[l]);return t}t.data={attributes:{}};let e=this.index;e!==null&&(t.data.index={type:e.array.constructor.name,array:Array.prototype.slice.call(e.array)});let i=this.attributes;for(let c in i){let l=i[c];t.data.attributes[c]=l.toJSON(t.data)}let s={},r=!1;for(let c in this.morphAttributes){let l=this.morphAttributes[c],f=[];for(let d=0,h=l.length;d<h;d++){let m=l[d];f.push(m.toJSON(t.data))}f.length>0&&(s[c]=f,r=!0)}r&&(t.data.morphAttributes=s,t.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(t.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(t.data.boundingSphere={center:a.center.toArray(),radius:a.radius}),t}clone(){return new this.constructor().copy(this)}copy(t){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let e={};this.name=t.name;let i=t.index;i!==null&&this.setIndex(i.clone(e));let s=t.attributes;for(let l in s){let f=s[l];this.setAttribute(l,f.clone(e))}let r=t.morphAttributes;for(let l in r){let f=[],d=r[l];for(let h=0,m=d.length;h<m;h++)f.push(d[h].clone(e));this.morphAttributes[l]=f}this.morphTargetsRelative=t.morphTargetsRelative;let o=t.groups;for(let l=0,f=o.length;l<f;l++){let d=o[l];this.addGroup(d.start,d.count,d.materialIndex)}let a=t.boundingBox;a!==null&&(this.boundingBox=a.clone());let c=t.boundingSphere;return c!==null&&(this.boundingSphere=c.clone()),this.drawRange.start=t.drawRange.start,this.drawRange.count=t.drawRange.count,this.userData=t.userData,this}dispose(){this.dispatchEvent({type:"dispose"})}},ru=new $t,xi=new bo,Zr=new Ci,ou=new D,jr=new D,Kr=new D,Jr=new D,cc=new D,Qr=new D,au=new D,$r=new D,Oe=class extends He{constructor(t=new pn,e=new bs){super(),this.isMesh=!0,this.type="Mesh",this.geometry=t,this.material=e,this.updateMorphTargets()}copy(t,e){return super.copy(t,e),t.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=t.morphTargetInfluences.slice()),t.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},t.morphTargetDictionary)),this.material=Array.isArray(t.material)?t.material.slice():t.material,this.geometry=t.geometry,this}updateMorphTargets(){let e=this.geometry.morphAttributes,i=Object.keys(e);if(i.length>0){let s=e[i[0]];if(s!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=s.length;r<o;r++){let a=s[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(t,e){let i=this.geometry,s=i.attributes.position,r=i.morphAttributes.position,o=i.morphTargetsRelative;e.fromBufferAttribute(s,t);let a=this.morphTargetInfluences;if(r&&a){Qr.set(0,0,0);for(let c=0,l=r.length;c<l;c++){let f=a[c],d=r[c];f!==0&&(cc.fromBufferAttribute(d,t),o?Qr.addScaledVector(cc,f):Qr.addScaledVector(cc.sub(e),f))}e.add(Qr)}return e}raycast(t,e){let i=this.geometry,s=this.material,r=this.matrixWorld;s!==void 0&&(i.boundingSphere===null&&i.computeBoundingSphere(),Zr.copy(i.boundingSphere),Zr.applyMatrix4(r),xi.copy(t.ray).recast(t.near),!(Zr.containsPoint(xi.origin)===!1&&(xi.intersectSphere(Zr,ou)===null||xi.origin.distanceToSquared(ou)>(t.far-t.near)**2))&&(ru.copy(r).invert(),xi.copy(t.ray).applyMatrix4(ru),!(i.boundingBox!==null&&xi.intersectsBox(i.boundingBox)===!1)&&this._computeIntersections(t,e,xi)))}_computeIntersections(t,e,i){let s,r=this.geometry,o=this.material,a=r.index,c=r.attributes.position,l=r.attributes.uv,f=r.attributes.uv1,d=r.attributes.normal,h=r.groups,m=r.drawRange;if(a!==null)if(Array.isArray(o))for(let x=0,_=h.length;x<_;x++){let u=h[x],p=o[u.materialIndex],b=Math.max(u.start,m.start),y=Math.min(a.count,Math.min(u.start+u.count,m.start+m.count));for(let A=b,R=y;A<R;A+=3){let T=a.getX(A),E=a.getX(A+1),C=a.getX(A+2);s=to(this,p,t,i,l,f,d,T,E,C),s&&(s.faceIndex=Math.floor(A/3),s.face.materialIndex=u.materialIndex,e.push(s))}}else{let x=Math.max(0,m.start),_=Math.min(a.count,m.start+m.count);for(let u=x,p=_;u<p;u+=3){let b=a.getX(u),y=a.getX(u+1),A=a.getX(u+2);s=to(this,o,t,i,l,f,d,b,y,A),s&&(s.faceIndex=Math.floor(u/3),e.push(s))}}else if(c!==void 0)if(Array.isArray(o))for(let x=0,_=h.length;x<_;x++){let u=h[x],p=o[u.materialIndex],b=Math.max(u.start,m.start),y=Math.min(c.count,Math.min(u.start+u.count,m.start+m.count));for(let A=b,R=y;A<R;A+=3){let T=A,E=A+1,C=A+2;s=to(this,p,t,i,l,f,d,T,E,C),s&&(s.faceIndex=Math.floor(A/3),s.face.materialIndex=u.materialIndex,e.push(s))}}else{let x=Math.max(0,m.start),_=Math.min(c.count,m.start+m.count);for(let u=x,p=_;u<p;u+=3){let b=u,y=u+1,A=u+2;s=to(this,o,t,i,l,f,d,b,y,A),s&&(s.faceIndex=Math.floor(u/3),e.push(s))}}}};function qp(n,t,e,i,s,r,o,a){let c;if(t.side===ze?c=i.intersectTriangle(o,r,s,!0,a):c=i.intersectTriangle(s,r,o,t.side===ei,a),c===null)return null;$r.copy(a),$r.applyMatrix4(n.matrixWorld);let l=e.ray.origin.distanceTo($r);return l<e.near||l>e.far?null:{distance:l,point:$r.clone(),object:n}}function to(n,t,e,i,s,r,o,a,c,l){n.getVertexPosition(a,jr),n.getVertexPosition(c,Kr),n.getVertexPosition(l,Jr);let f=qp(n,t,e,i,jr,Kr,Jr,au);if(f){let d=new D;Si.getBarycoord(au,jr,Kr,Jr,d),s&&(f.uv=Si.getInterpolatedAttribute(s,a,c,l,d,new yt)),r&&(f.uv1=Si.getInterpolatedAttribute(r,a,c,l,d,new yt)),o&&(f.normal=Si.getInterpolatedAttribute(o,a,c,l,d,new D),f.normal.dot(i.direction)>0&&f.normal.multiplyScalar(-1));let h={a,b:c,c:l,normal:new D,materialIndex:0};Si.getNormal(jr,Kr,Jr,h.normal),f.face=h,f.barycoord=d}return f}var hr=class n extends pn{constructor(t=1,e=1,i=1,s=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:t,height:e,depth:i,widthSegments:s,heightSegments:r,depthSegments:o};let a=this;s=Math.floor(s),r=Math.floor(r),o=Math.floor(o);let c=[],l=[],f=[],d=[],h=0,m=0;x("z","y","x",-1,-1,i,e,t,o,r,0),x("z","y","x",1,-1,i,e,-t,o,r,1),x("x","z","y",1,1,t,i,e,s,o,2),x("x","z","y",1,-1,t,i,-e,s,o,3),x("x","y","z",1,-1,t,e,i,s,r,4),x("x","y","z",-1,-1,t,e,-i,s,r,5),this.setIndex(c),this.setAttribute("position",new be(l,3)),this.setAttribute("normal",new be(f,3)),this.setAttribute("uv",new be(d,2));function x(_,u,p,b,y,A,R,T,E,C,N){let g=A/E,S=R/C,F=A/2,V=R/2,q=T/2,et=E+1,B=C+1,it=0,Z=0,gt=new D;for(let _t=0;_t<B;_t++){let bt=_t*S-V;for(let Gt=0;Gt<et;Gt++){let Wt=Gt*g-F;gt[_]=Wt*b,gt[u]=bt*y,gt[p]=q,l.push(gt.x,gt.y,gt.z),gt[_]=0,gt[u]=0,gt[p]=T>0?1:-1,f.push(gt.x,gt.y,gt.z),d.push(Gt/E),d.push(1-_t/C),it+=1}}for(let _t=0;_t<C;_t++)for(let bt=0;bt<E;bt++){let Gt=h+bt+et*_t,Wt=h+bt+et*(_t+1),J=h+(bt+1)+et*(_t+1),st=h+(bt+1)+et*_t;c.push(Gt,Wt,st),c.push(Wt,J,st),Z+=6}a.addGroup(m,Z,N),m+=Z,h+=it}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.width,t.height,t.depth,t.widthSegments,t.heightSegments,t.depthSegments)}};function Ss(n){let t={};for(let e in n){t[e]={};for(let i in n[e]){let s=n[e][i];s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)?s.isRenderTargetTexture?(console.warn("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),t[e][i]=null):t[e][i]=s.clone():Array.isArray(s)?t[e][i]=s.slice():t[e][i]=s}}return t}function De(n){let t={};for(let e=0;e<n.length;e++){let i=Ss(n[e]);for(let s in i)t[s]=i[s]}return t}function Yp(n){let t=[];for(let e=0;e<n.length;e++)t.push(n[e].clone());return t}function nd(n){let t=n.getRenderTarget();return t===null?n.outputColorSpace:t.isXRRenderTarget===!0?t.texture.colorSpace:Jt.workingColorSpace}var wn={clone:Ss,merge:De},Zp=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,jp=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,te=class extends Ri{constructor(t){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Zp,this.fragmentShader=jp,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,t!==void 0&&this.setValues(t)}copy(t){return super.copy(t),this.fragmentShader=t.fragmentShader,this.vertexShader=t.vertexShader,this.uniforms=Ss(t.uniforms),this.uniformsGroups=Yp(t.uniformsGroups),this.defines=Object.assign({},t.defines),this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.fog=t.fog,this.lights=t.lights,this.clipping=t.clipping,this.extensions=Object.assign({},t.extensions),this.glslVersion=t.glslVersion,this}toJSON(t){let e=super.toJSON(t);e.glslVersion=this.glslVersion,e.uniforms={};for(let s in this.uniforms){let o=this.uniforms[s].value;o&&o.isTexture?e.uniforms[s]={type:"t",value:o.toJSON(t).uuid}:o&&o.isColor?e.uniforms[s]={type:"c",value:o.getHex()}:o&&o.isVector2?e.uniforms[s]={type:"v2",value:o.toArray()}:o&&o.isVector3?e.uniforms[s]={type:"v3",value:o.toArray()}:o&&o.isVector4?e.uniforms[s]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?e.uniforms[s]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?e.uniforms[s]={type:"m4",value:o.toArray()}:e.uniforms[s]={value:o}}Object.keys(this.defines).length>0&&(e.defines=this.defines),e.vertexShader=this.vertexShader,e.fragmentShader=this.fragmentShader,e.lights=this.lights,e.clipping=this.clipping;let i={};for(let s in this.extensions)this.extensions[s]===!0&&(i[s]=!0);return Object.keys(i).length>0&&(e.extensions=i),e}},Ao=class extends He{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new $t,this.projectionMatrix=new $t,this.projectionMatrixInverse=new $t,this.coordinateSystem=On}copy(t,e){return super.copy(t,e),this.matrixWorldInverse.copy(t.matrixWorldInverse),this.projectionMatrix.copy(t.projectionMatrix),this.projectionMatrixInverse.copy(t.projectionMatrixInverse),this.coordinateSystem=t.coordinateSystem,this}getWorldDirection(t){return super.getWorldDirection(t).negate()}updateMatrixWorld(t){super.updateMatrixWorld(t),this.matrixWorldInverse.copy(this.matrixWorld).invert()}updateWorldMatrix(t,e){super.updateWorldMatrix(t,e),this.matrixWorldInverse.copy(this.matrixWorld).invert()}clone(){return new this.constructor().copy(this)}},Jn=new D,cu=new yt,lu=new yt,Ne=class extends Ao{constructor(t=50,e=1,i=.1,s=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=t,this.zoom=1,this.near=i,this.far=s,this.focus=10,this.aspect=e,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(t,e){return super.copy(t,e),this.fov=t.fov,this.zoom=t.zoom,this.near=t.near,this.far=t.far,this.focus=t.focus,this.aspect=t.aspect,this.view=t.view===null?null:Object.assign({},t.view),this.filmGauge=t.filmGauge,this.filmOffset=t.filmOffset,this}setFocalLength(t){let e=.5*this.getFilmHeight()/t;this.fov=cr*2*Math.atan(e),this.updateProjectionMatrix()}getFocalLength(){let t=Math.tan(sr*.5*this.fov);return .5*this.getFilmHeight()/t}getEffectiveFOV(){return cr*2*Math.atan(Math.tan(sr*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(t,e,i){Jn.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),e.set(Jn.x,Jn.y).multiplyScalar(-t/Jn.z),Jn.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),i.set(Jn.x,Jn.y).multiplyScalar(-t/Jn.z)}getViewSize(t,e){return this.getViewBounds(t,cu,lu),e.subVectors(lu,cu)}setViewOffset(t,e,i,s,r,o){this.aspect=t/e,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=t,this.view.fullHeight=e,this.view.offsetX=i,this.view.offsetY=s,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let t=this.near,e=t*Math.tan(sr*.5*this.fov)/this.zoom,i=2*e,s=this.aspect*i,r=-.5*s,o=this.view;if(this.view!==null&&this.view.enabled){let c=o.fullWidth,l=o.fullHeight;r+=o.offsetX*s/c,e-=o.offsetY*i/l,s*=o.width/c,i*=o.height/l}let a=this.filmOffset;a!==0&&(r+=t*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+s,e,e-i,t,this.far,this.coordinateSystem),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(t){let e=super.toJSON(t);return e.object.fov=this.fov,e.object.zoom=this.zoom,e.object.near=this.near,e.object.far=this.far,e.object.focus=this.focus,e.object.aspect=this.aspect,this.view!==null&&(e.object.view=Object.assign({},this.view)),e.object.filmGauge=this.filmGauge,e.object.filmOffset=this.filmOffset,e}},as=-90,cs=1,ol=class extends He{constructor(t,e,i){super(),this.type="CubeCamera",this.renderTarget=i,this.coordinateSystem=null,this.activeMipmapLevel=0;let s=new Ne(as,cs,t,e);s.layers=this.layers,this.add(s);let r=new Ne(as,cs,t,e);r.layers=this.layers,this.add(r);let o=new Ne(as,cs,t,e);o.layers=this.layers,this.add(o);let a=new Ne(as,cs,t,e);a.layers=this.layers,this.add(a);let c=new Ne(as,cs,t,e);c.layers=this.layers,this.add(c);let l=new Ne(as,cs,t,e);l.layers=this.layers,this.add(l)}updateCoordinateSystem(){let t=this.coordinateSystem,e=this.children.concat(),[i,s,r,o,a,c]=e;for(let l of e)this.remove(l);if(t===On)i.up.set(0,1,0),i.lookAt(1,0,0),s.up.set(0,1,0),s.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),c.up.set(0,1,0),c.lookAt(0,0,-1);else if(t===vo)i.up.set(0,-1,0),i.lookAt(-1,0,0),s.up.set(0,-1,0),s.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),c.up.set(0,-1,0),c.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+t);for(let l of e)this.add(l),l.updateMatrixWorld()}update(t,e){this.parent===null&&this.updateMatrixWorld();let{renderTarget:i,activeMipmapLevel:s}=this;this.coordinateSystem!==t.coordinateSystem&&(this.coordinateSystem=t.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,c,l,f]=this.children,d=t.getRenderTarget(),h=t.getActiveCubeFace(),m=t.getActiveMipmapLevel(),x=t.xr.enabled;t.xr.enabled=!1;let _=i.texture.generateMipmaps;i.texture.generateMipmaps=!1,t.setRenderTarget(i,0,s),t.render(e,r),t.setRenderTarget(i,1,s),t.render(e,o),t.setRenderTarget(i,2,s),t.render(e,a),t.setRenderTarget(i,3,s),t.render(e,c),t.setRenderTarget(i,4,s),t.render(e,l),i.texture.generateMipmaps=_,t.setRenderTarget(i,5,s),t.render(e,f),t.setRenderTarget(d,h,m),t.xr.enabled=x,i.texture.needsPMREMUpdate=!0}},Eo=class extends Pe{constructor(t,e,i,s,r,o,a,c,l,f){t=t!==void 0?t:[],e=e!==void 0?e:vs,super(t,e,i,s,r,o,a,c,l,f),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(t){this.image=t}},al=class extends fe{constructor(t=1,e={}){super(t,t,e),this.isWebGLCubeRenderTarget=!0;let i={width:t,height:t,depth:1},s=[i,i,i,i,i,i];this.texture=new Eo(s,e.mapping,e.wrapS,e.wrapT,e.magFilter,e.minFilter,e.format,e.type,e.anisotropy,e.colorSpace),this.texture.isRenderTargetTexture=!0,this.texture.generateMipmaps=e.generateMipmaps!==void 0?e.generateMipmaps:!1,this.texture.minFilter=e.minFilter!==void 0?e.minFilter:je}fromEquirectangularTexture(t,e){this.texture.type=e.type,this.texture.colorSpace=e.colorSpace,this.texture.generateMipmaps=e.generateMipmaps,this.texture.minFilter=e.minFilter,this.texture.magFilter=e.magFilter;let i={uniforms:{tEquirect:{value:null}},vertexShader:`

				varying vec3 vWorldDirection;

				vec3 transformDirection( in vec3 dir, in mat4 matrix ) {

					return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );

				}

				void main() {

					vWorldDirection = transformDirection( position, modelMatrix );

					#include <begin_vertex>
					#include <project_vertex>

				}
			`,fragmentShader:`

				uniform sampler2D tEquirect;

				varying vec3 vWorldDirection;

				#include <common>

				void main() {

					vec3 direction = normalize( vWorldDirection );

					vec2 sampleUV = equirectUv( direction );

					gl_FragColor = texture2D( tEquirect, sampleUV );

				}
			`},s=new hr(5,5,5),r=new te({name:"CubemapFromEquirect",uniforms:Ss(i.uniforms),vertexShader:i.vertexShader,fragmentShader:i.fragmentShader,side:ze,blending:Sn});r.uniforms.tEquirect.value=e;let o=new Oe(s,r),a=e.minFilter;return e.minFilter===Ei&&(e.minFilter=je),new ol(1,10,this).update(t,o),e.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(t,e,i,s){let r=t.getRenderTarget();for(let o=0;o<6;o++)t.setRenderTarget(this,o),t.clear(e,i,s);t.setRenderTarget(r)}},lc=new D,Kp=new D,Jp=new zt,dn=class{constructor(t=new D(1,0,0),e=0){this.isPlane=!0,this.normal=t,this.constant=e}set(t,e){return this.normal.copy(t),this.constant=e,this}setComponents(t,e,i,s){return this.normal.set(t,e,i),this.constant=s,this}setFromNormalAndCoplanarPoint(t,e){return this.normal.copy(t),this.constant=-e.dot(this.normal),this}setFromCoplanarPoints(t,e,i){let s=lc.subVectors(i,e).cross(Kp.subVectors(t,e)).normalize();return this.setFromNormalAndCoplanarPoint(s,t),this}copy(t){return this.normal.copy(t.normal),this.constant=t.constant,this}normalize(){let t=1/this.normal.length();return this.normal.multiplyScalar(t),this.constant*=t,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(t){return this.normal.dot(t)+this.constant}distanceToSphere(t){return this.distanceToPoint(t.center)-t.radius}projectPoint(t,e){return e.copy(t).addScaledVector(this.normal,-this.distanceToPoint(t))}intersectLine(t,e){let i=t.delta(lc),s=this.normal.dot(i);if(s===0)return this.distanceToPoint(t.start)===0?e.copy(t.start):null;let r=-(t.start.dot(this.normal)+this.constant)/s;return r<0||r>1?null:e.copy(t.start).addScaledVector(i,r)}intersectsLine(t){let e=this.distanceToPoint(t.start),i=this.distanceToPoint(t.end);return e<0&&i>0||i<0&&e>0}intersectsBox(t){return t.intersectsPlane(this)}intersectsSphere(t){return t.intersectsPlane(this)}coplanarPoint(t){return t.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(t,e){let i=e||Jp.getNormalMatrix(t),s=this.coplanarPoint(lc).applyMatrix4(t),r=this.normal.applyMatrix3(i).normalize();return this.constant=-s.dot(r),this}translate(t){return this.constant-=t.dot(this.normal),this}equals(t){return t.normal.equals(this.normal)&&t.constant===this.constant}clone(){return new this.constructor().copy(this)}},vi=new Ci,eo=new D,ur=class{constructor(t=new dn,e=new dn,i=new dn,s=new dn,r=new dn,o=new dn){this.planes=[t,e,i,s,r,o]}set(t,e,i,s,r,o){let a=this.planes;return a[0].copy(t),a[1].copy(e),a[2].copy(i),a[3].copy(s),a[4].copy(r),a[5].copy(o),this}copy(t){let e=this.planes;for(let i=0;i<6;i++)e[i].copy(t.planes[i]);return this}setFromProjectionMatrix(t,e=On){let i=this.planes,s=t.elements,r=s[0],o=s[1],a=s[2],c=s[3],l=s[4],f=s[5],d=s[6],h=s[7],m=s[8],x=s[9],_=s[10],u=s[11],p=s[12],b=s[13],y=s[14],A=s[15];if(i[0].setComponents(c-r,h-l,u-m,A-p).normalize(),i[1].setComponents(c+r,h+l,u+m,A+p).normalize(),i[2].setComponents(c+o,h+f,u+x,A+b).normalize(),i[3].setComponents(c-o,h-f,u-x,A-b).normalize(),i[4].setComponents(c-a,h-d,u-_,A-y).normalize(),e===On)i[5].setComponents(c+a,h+d,u+_,A+y).normalize();else if(e===vo)i[5].setComponents(a,d,_,y).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+e);return this}intersectsObject(t){if(t.boundingSphere!==void 0)t.boundingSphere===null&&t.computeBoundingSphere(),vi.copy(t.boundingSphere).applyMatrix4(t.matrixWorld);else{let e=t.geometry;e.boundingSphere===null&&e.computeBoundingSphere(),vi.copy(e.boundingSphere).applyMatrix4(t.matrixWorld)}return this.intersectsSphere(vi)}intersectsSprite(t){return vi.center.set(0,0,0),vi.radius=.7071067811865476,vi.applyMatrix4(t.matrixWorld),this.intersectsSphere(vi)}intersectsSphere(t){let e=this.planes,i=t.center,s=-t.radius;for(let r=0;r<6;r++)if(e[r].distanceToPoint(i)<s)return!1;return!0}intersectsBox(t){let e=this.planes;for(let i=0;i<6;i++){let s=e[i];if(eo.x=s.normal.x>0?t.max.x:t.min.x,eo.y=s.normal.y>0?t.max.y:t.min.y,eo.z=s.normal.z>0?t.max.z:t.min.z,s.distanceToPoint(eo)<0)return!1}return!0}containsPoint(t){let e=this.planes;for(let i=0;i<6;i++)if(e[i].distanceToPoint(t)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};function id(){let n=null,t=!1,e=null,i=null;function s(r,o){e(r,o),i=n.requestAnimationFrame(s)}return{start:function(){t!==!0&&e!==null&&(i=n.requestAnimationFrame(s),t=!0)},stop:function(){n.cancelAnimationFrame(i),t=!1},setAnimationLoop:function(r){e=r},setContext:function(r){n=r}}}function Qp(n){let t=new WeakMap;function e(a,c){let l=a.array,f=a.usage,d=l.byteLength,h=n.createBuffer();n.bindBuffer(c,h),n.bufferData(c,l,f),a.onUploadCallback();let m;if(l instanceof Float32Array)m=n.FLOAT;else if(l instanceof Uint16Array)a.isFloat16BufferAttribute?m=n.HALF_FLOAT:m=n.UNSIGNED_SHORT;else if(l instanceof Int16Array)m=n.SHORT;else if(l instanceof Uint32Array)m=n.UNSIGNED_INT;else if(l instanceof Int32Array)m=n.INT;else if(l instanceof Int8Array)m=n.BYTE;else if(l instanceof Uint8Array)m=n.UNSIGNED_BYTE;else if(l instanceof Uint8ClampedArray)m=n.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+l);return{buffer:h,type:m,bytesPerElement:l.BYTES_PER_ELEMENT,version:a.version,size:d}}function i(a,c,l){let f=c.array,d=c.updateRanges;if(n.bindBuffer(l,a),d.length===0)n.bufferSubData(l,0,f);else{d.sort((m,x)=>m.start-x.start);let h=0;for(let m=1;m<d.length;m++){let x=d[h],_=d[m];_.start<=x.start+x.count+1?x.count=Math.max(x.count,_.start+_.count-x.start):(++h,d[h]=_)}d.length=h+1;for(let m=0,x=d.length;m<x;m++){let _=d[m];n.bufferSubData(l,_.start*f.BYTES_PER_ELEMENT,f,_.start,_.count)}c.clearUpdateRanges()}c.onUploadCallback()}function s(a){return a.isInterleavedBufferAttribute&&(a=a.data),t.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let c=t.get(a);c&&(n.deleteBuffer(c.buffer),t.delete(a))}function o(a,c){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let f=t.get(a);(!f||f.version<a.version)&&t.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let l=t.get(a);if(l===void 0)t.set(a,e(a,c));else if(l.version<a.version){if(l.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");i(l.buffer,a,c),l.version=a.version}}return{get:s,remove:r,update:o}}var To=class n extends pn{constructor(t=1,e=1,i=1,s=1){super(),this.type="PlaneGeometry",this.parameters={width:t,height:e,widthSegments:i,heightSegments:s};let r=t/2,o=e/2,a=Math.floor(i),c=Math.floor(s),l=a+1,f=c+1,d=t/a,h=e/c,m=[],x=[],_=[],u=[];for(let p=0;p<f;p++){let b=p*h-o;for(let y=0;y<l;y++){let A=y*d-r;x.push(A,-b,0),_.push(0,0,1),u.push(y/a),u.push(1-p/c)}}for(let p=0;p<c;p++)for(let b=0;b<a;b++){let y=b+l*p,A=b+l*(p+1),R=b+1+l*(p+1),T=b+1+l*p;m.push(y,A,T),m.push(A,R,T)}this.setIndex(m),this.setAttribute("position",new be(x,3)),this.setAttribute("normal",new be(_,3)),this.setAttribute("uv",new be(u,2))}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.width,t.height,t.widthSegments,t.heightSegments)}},$p=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,tm=`#ifdef USE_ALPHAHASH
	const float ALPHA_HASH_SCALE = 0.05;
	float hash2D( vec2 value ) {
		return fract( 1.0e4 * sin( 17.0 * value.x + 0.1 * value.y ) * ( 0.1 + abs( sin( 13.0 * value.y + value.x ) ) ) );
	}
	float hash3D( vec3 value ) {
		return hash2D( vec2( hash2D( value.xy ), value.z ) );
	}
	float getAlphaHashThreshold( vec3 position ) {
		float maxDeriv = max(
			length( dFdx( position.xyz ) ),
			length( dFdy( position.xyz ) )
		);
		float pixScale = 1.0 / ( ALPHA_HASH_SCALE * maxDeriv );
		vec2 pixScales = vec2(
			exp2( floor( log2( pixScale ) ) ),
			exp2( ceil( log2( pixScale ) ) )
		);
		vec2 alpha = vec2(
			hash3D( floor( pixScales.x * position.xyz ) ),
			hash3D( floor( pixScales.y * position.xyz ) )
		);
		float lerpFactor = fract( log2( pixScale ) );
		float x = ( 1.0 - lerpFactor ) * alpha.x + lerpFactor * alpha.y;
		float a = min( lerpFactor, 1.0 - lerpFactor );
		vec3 cases = vec3(
			x * x / ( 2.0 * a * ( 1.0 - a ) ),
			( x - 0.5 * a ) / ( 1.0 - a ),
			1.0 - ( ( 1.0 - x ) * ( 1.0 - x ) / ( 2.0 * a * ( 1.0 - a ) ) )
		);
		float threshold = ( x < ( 1.0 - a ) )
			? ( ( x < a ) ? cases.x : cases.y )
			: cases.z;
		return clamp( threshold , 1.0e-6, 1.0 );
	}
#endif`,em=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,nm=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,im=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,sm=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,rm=`#ifdef USE_AOMAP
	float ambientOcclusion = ( texture2D( aoMap, vAoMapUv ).r - 1.0 ) * aoMapIntensity + 1.0;
	reflectedLight.indirectDiffuse *= ambientOcclusion;
	#if defined( USE_CLEARCOAT ) 
		clearcoatSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_SHEEN ) 
		sheenSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_ENVMAP ) && defined( STANDARD )
		float dotNV = saturate( dot( geometryNormal, geometryViewDir ) );
		reflectedLight.indirectSpecular *= computeSpecularOcclusion( dotNV, ambientOcclusion, material.roughness );
	#endif
#endif`,om=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,am=`#ifdef USE_BATCHING
	#if ! defined( GL_ANGLE_multi_draw )
	#define gl_DrawID _gl_DrawID
	uniform int _gl_DrawID;
	#endif
	uniform highp sampler2D batchingTexture;
	uniform highp usampler2D batchingIdTexture;
	mat4 getBatchingMatrix( const in float i ) {
		int size = textureSize( batchingTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( batchingTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( batchingTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( batchingTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( batchingTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
	float getIndirectIndex( const in int i ) {
		int size = textureSize( batchingIdTexture, 0 ).x;
		int x = i % size;
		int y = i / size;
		return float( texelFetch( batchingIdTexture, ivec2( x, y ), 0 ).r );
	}
#endif
#ifdef USE_BATCHING_COLOR
	uniform sampler2D batchingColorTexture;
	vec3 getBatchingColor( const in float i ) {
		int size = textureSize( batchingColorTexture, 0 ).x;
		int j = int( i );
		int x = j % size;
		int y = j / size;
		return texelFetch( batchingColorTexture, ivec2( x, y ), 0 ).rgb;
	}
#endif`,cm=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,lm=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,hm=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,um=`float G_BlinnPhong_Implicit( ) {
	return 0.25;
}
float D_BlinnPhong( const in float shininess, const in float dotNH ) {
	return RECIPROCAL_PI * ( shininess * 0.5 + 1.0 ) * pow( dotNH, shininess );
}
vec3 BRDF_BlinnPhong( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in vec3 specularColor, const in float shininess ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( specularColor, 1.0, dotVH );
	float G = G_BlinnPhong_Implicit( );
	float D = D_BlinnPhong( shininess, dotNH );
	return F * ( G * D );
} // validated`,dm=`#ifdef USE_IRIDESCENCE
	const mat3 XYZ_TO_REC709 = mat3(
		 3.2404542, -0.9692660,  0.0556434,
		-1.5371385,  1.8760108, -0.2040259,
		-0.4985314,  0.0415560,  1.0572252
	);
	vec3 Fresnel0ToIor( vec3 fresnel0 ) {
		vec3 sqrtF0 = sqrt( fresnel0 );
		return ( vec3( 1.0 ) + sqrtF0 ) / ( vec3( 1.0 ) - sqrtF0 );
	}
	vec3 IorToFresnel0( vec3 transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - vec3( incidentIor ) ) / ( transmittedIor + vec3( incidentIor ) ) );
	}
	float IorToFresnel0( float transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - incidentIor ) / ( transmittedIor + incidentIor ));
	}
	vec3 evalSensitivity( float OPD, vec3 shift ) {
		float phase = 2.0 * PI * OPD * 1.0e-9;
		vec3 val = vec3( 5.4856e-13, 4.4201e-13, 5.2481e-13 );
		vec3 pos = vec3( 1.6810e+06, 1.7953e+06, 2.2084e+06 );
		vec3 var = vec3( 4.3278e+09, 9.3046e+09, 6.6121e+09 );
		vec3 xyz = val * sqrt( 2.0 * PI * var ) * cos( pos * phase + shift ) * exp( - pow2( phase ) * var );
		xyz.x += 9.7470e-14 * sqrt( 2.0 * PI * 4.5282e+09 ) * cos( 2.2399e+06 * phase + shift[ 0 ] ) * exp( - 4.5282e+09 * pow2( phase ) );
		xyz /= 1.0685e-7;
		vec3 rgb = XYZ_TO_REC709 * xyz;
		return rgb;
	}
	vec3 evalIridescence( float outsideIOR, float eta2, float cosTheta1, float thinFilmThickness, vec3 baseF0 ) {
		vec3 I;
		float iridescenceIOR = mix( outsideIOR, eta2, smoothstep( 0.0, 0.03, thinFilmThickness ) );
		float sinTheta2Sq = pow2( outsideIOR / iridescenceIOR ) * ( 1.0 - pow2( cosTheta1 ) );
		float cosTheta2Sq = 1.0 - sinTheta2Sq;
		if ( cosTheta2Sq < 0.0 ) {
			return vec3( 1.0 );
		}
		float cosTheta2 = sqrt( cosTheta2Sq );
		float R0 = IorToFresnel0( iridescenceIOR, outsideIOR );
		float R12 = F_Schlick( R0, 1.0, cosTheta1 );
		float T121 = 1.0 - R12;
		float phi12 = 0.0;
		if ( iridescenceIOR < outsideIOR ) phi12 = PI;
		float phi21 = PI - phi12;
		vec3 baseIOR = Fresnel0ToIor( clamp( baseF0, 0.0, 0.9999 ) );		vec3 R1 = IorToFresnel0( baseIOR, iridescenceIOR );
		vec3 R23 = F_Schlick( R1, 1.0, cosTheta2 );
		vec3 phi23 = vec3( 0.0 );
		if ( baseIOR[ 0 ] < iridescenceIOR ) phi23[ 0 ] = PI;
		if ( baseIOR[ 1 ] < iridescenceIOR ) phi23[ 1 ] = PI;
		if ( baseIOR[ 2 ] < iridescenceIOR ) phi23[ 2 ] = PI;
		float OPD = 2.0 * iridescenceIOR * thinFilmThickness * cosTheta2;
		vec3 phi = vec3( phi21 ) + phi23;
		vec3 R123 = clamp( R12 * R23, 1e-5, 0.9999 );
		vec3 r123 = sqrt( R123 );
		vec3 Rs = pow2( T121 ) * R23 / ( vec3( 1.0 ) - R123 );
		vec3 C0 = R12 + Rs;
		I = C0;
		vec3 Cm = Rs - T121;
		for ( int m = 1; m <= 2; ++ m ) {
			Cm *= r123;
			vec3 Sm = 2.0 * evalSensitivity( float( m ) * OPD, float( m ) * phi );
			I += Cm * Sm;
		}
		return max( I, vec3( 0.0 ) );
	}
#endif`,fm=`#ifdef USE_BUMPMAP
	uniform sampler2D bumpMap;
	uniform float bumpScale;
	vec2 dHdxy_fwd() {
		vec2 dSTdx = dFdx( vBumpMapUv );
		vec2 dSTdy = dFdy( vBumpMapUv );
		float Hll = bumpScale * texture2D( bumpMap, vBumpMapUv ).x;
		float dBx = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdx ).x - Hll;
		float dBy = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdy ).x - Hll;
		return vec2( dBx, dBy );
	}
	vec3 perturbNormalArb( vec3 surf_pos, vec3 surf_norm, vec2 dHdxy, float faceDirection ) {
		vec3 vSigmaX = normalize( dFdx( surf_pos.xyz ) );
		vec3 vSigmaY = normalize( dFdy( surf_pos.xyz ) );
		vec3 vN = surf_norm;
		vec3 R1 = cross( vSigmaY, vN );
		vec3 R2 = cross( vN, vSigmaX );
		float fDet = dot( vSigmaX, R1 ) * faceDirection;
		vec3 vGrad = sign( fDet ) * ( dHdxy.x * R1 + dHdxy.y * R2 );
		return normalize( abs( fDet ) * surf_norm - vGrad );
	}
#endif`,pm=`#if NUM_CLIPPING_PLANES > 0
	vec4 plane;
	#ifdef ALPHA_TO_COVERAGE
		float distanceToPlane, distanceGradient;
		float clipOpacity = 1.0;
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
			distanceGradient = fwidth( distanceToPlane ) / 2.0;
			clipOpacity *= smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			if ( clipOpacity == 0.0 ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			float unionClipOpacity = 1.0;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
				distanceGradient = fwidth( distanceToPlane ) / 2.0;
				unionClipOpacity *= 1.0 - smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			}
			#pragma unroll_loop_end
			clipOpacity *= 1.0 - unionClipOpacity;
		#endif
		diffuseColor.a *= clipOpacity;
		if ( diffuseColor.a == 0.0 ) discard;
	#else
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			if ( dot( vClipPosition, plane.xyz ) > plane.w ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			bool clipped = true;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				clipped = ( dot( vClipPosition, plane.xyz ) > plane.w ) && clipped;
			}
			#pragma unroll_loop_end
			if ( clipped ) discard;
		#endif
	#endif
#endif`,mm=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,gm=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,xm=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,vm=`#if defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#elif defined( USE_COLOR )
	diffuseColor.rgb *= vColor;
#endif`,_m=`#if defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#elif defined( USE_COLOR )
	varying vec3 vColor;
#endif`,ym=`#if defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#elif defined( USE_COLOR ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec3 vColor;
#endif`,Mm=`#if defined( USE_COLOR_ALPHA )
	vColor = vec4( 1.0 );
#elif defined( USE_COLOR ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	vColor = vec3( 1.0 );
#endif
#ifdef USE_COLOR
	vColor *= color;
#endif
#ifdef USE_INSTANCING_COLOR
	vColor.xyz *= instanceColor.xyz;
#endif
#ifdef USE_BATCHING_COLOR
	vec3 batchingColor = getBatchingColor( getIndirectIndex( gl_DrawID ) );
	vColor.xyz *= batchingColor.xyz;
#endif`,bm=`#define PI 3.141592653589793
#define PI2 6.283185307179586
#define PI_HALF 1.5707963267948966
#define RECIPROCAL_PI 0.3183098861837907
#define RECIPROCAL_PI2 0.15915494309189535
#define EPSILON 1e-6
#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
#define whiteComplement( a ) ( 1.0 - saturate( a ) )
float pow2( const in float x ) { return x*x; }
vec3 pow2( const in vec3 x ) { return x*x; }
float pow3( const in float x ) { return x*x*x; }
float pow4( const in float x ) { float x2 = x*x; return x2*x2; }
float max3( const in vec3 v ) { return max( max( v.x, v.y ), v.z ); }
float average( const in vec3 v ) { return dot( v, vec3( 0.3333333 ) ); }
highp float rand( const in vec2 uv ) {
	const highp float a = 12.9898, b = 78.233, c = 43758.5453;
	highp float dt = dot( uv.xy, vec2( a,b ) ), sn = mod( dt, PI );
	return fract( sin( sn ) * c );
}
#ifdef HIGH_PRECISION
	float precisionSafeLength( vec3 v ) { return length( v ); }
#else
	float precisionSafeLength( vec3 v ) {
		float maxComponent = max3( abs( v ) );
		return length( v / maxComponent ) * maxComponent;
	}
#endif
struct IncidentLight {
	vec3 color;
	vec3 direction;
	bool visible;
};
struct ReflectedLight {
	vec3 directDiffuse;
	vec3 directSpecular;
	vec3 indirectDiffuse;
	vec3 indirectSpecular;
};
#ifdef USE_ALPHAHASH
	varying vec3 vPosition;
#endif
vec3 transformDirection( in vec3 dir, in mat4 matrix ) {
	return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );
}
vec3 inverseTransformDirection( in vec3 dir, in mat4 matrix ) {
	return normalize( ( vec4( dir, 0.0 ) * matrix ).xyz );
}
mat3 transposeMat3( const in mat3 m ) {
	mat3 tmp;
	tmp[ 0 ] = vec3( m[ 0 ].x, m[ 1 ].x, m[ 2 ].x );
	tmp[ 1 ] = vec3( m[ 0 ].y, m[ 1 ].y, m[ 2 ].y );
	tmp[ 2 ] = vec3( m[ 0 ].z, m[ 1 ].z, m[ 2 ].z );
	return tmp;
}
bool isPerspectiveMatrix( mat4 m ) {
	return m[ 2 ][ 3 ] == - 1.0;
}
vec2 equirectUv( in vec3 dir ) {
	float u = atan( dir.z, dir.x ) * RECIPROCAL_PI2 + 0.5;
	float v = asin( clamp( dir.y, - 1.0, 1.0 ) ) * RECIPROCAL_PI + 0.5;
	return vec2( u, v );
}
vec3 BRDF_Lambert( const in vec3 diffuseColor ) {
	return RECIPROCAL_PI * diffuseColor;
}
vec3 F_Schlick( const in vec3 f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
}
float F_Schlick( const in float f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
} // validated`,Sm=`#ifdef ENVMAP_TYPE_CUBE_UV
	#define cubeUV_minMipLevel 4.0
	#define cubeUV_minTileSize 16.0
	float getFace( vec3 direction ) {
		vec3 absDirection = abs( direction );
		float face = - 1.0;
		if ( absDirection.x > absDirection.z ) {
			if ( absDirection.x > absDirection.y )
				face = direction.x > 0.0 ? 0.0 : 3.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		} else {
			if ( absDirection.z > absDirection.y )
				face = direction.z > 0.0 ? 2.0 : 5.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		}
		return face;
	}
	vec2 getUV( vec3 direction, float face ) {
		vec2 uv;
		if ( face == 0.0 ) {
			uv = vec2( direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 1.0 ) {
			uv = vec2( - direction.x, - direction.z ) / abs( direction.y );
		} else if ( face == 2.0 ) {
			uv = vec2( - direction.x, direction.y ) / abs( direction.z );
		} else if ( face == 3.0 ) {
			uv = vec2( - direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 4.0 ) {
			uv = vec2( - direction.x, direction.z ) / abs( direction.y );
		} else {
			uv = vec2( direction.x, direction.y ) / abs( direction.z );
		}
		return 0.5 * ( uv + 1.0 );
	}
	vec3 bilinearCubeUV( sampler2D envMap, vec3 direction, float mipInt ) {
		float face = getFace( direction );
		float filterInt = max( cubeUV_minMipLevel - mipInt, 0.0 );
		mipInt = max( mipInt, cubeUV_minMipLevel );
		float faceSize = exp2( mipInt );
		highp vec2 uv = getUV( direction, face ) * ( faceSize - 2.0 ) + 1.0;
		if ( face > 2.0 ) {
			uv.y += faceSize;
			face -= 3.0;
		}
		uv.x += face * faceSize;
		uv.x += filterInt * 3.0 * cubeUV_minTileSize;
		uv.y += 4.0 * ( exp2( CUBEUV_MAX_MIP ) - faceSize );
		uv.x *= CUBEUV_TEXEL_WIDTH;
		uv.y *= CUBEUV_TEXEL_HEIGHT;
		#ifdef texture2DGradEXT
			return texture2DGradEXT( envMap, uv, vec2( 0.0 ), vec2( 0.0 ) ).rgb;
		#else
			return texture2D( envMap, uv ).rgb;
		#endif
	}
	#define cubeUV_r0 1.0
	#define cubeUV_m0 - 2.0
	#define cubeUV_r1 0.8
	#define cubeUV_m1 - 1.0
	#define cubeUV_r4 0.4
	#define cubeUV_m4 2.0
	#define cubeUV_r5 0.305
	#define cubeUV_m5 3.0
	#define cubeUV_r6 0.21
	#define cubeUV_m6 4.0
	float roughnessToMip( float roughness ) {
		float mip = 0.0;
		if ( roughness >= cubeUV_r1 ) {
			mip = ( cubeUV_r0 - roughness ) * ( cubeUV_m1 - cubeUV_m0 ) / ( cubeUV_r0 - cubeUV_r1 ) + cubeUV_m0;
		} else if ( roughness >= cubeUV_r4 ) {
			mip = ( cubeUV_r1 - roughness ) * ( cubeUV_m4 - cubeUV_m1 ) / ( cubeUV_r1 - cubeUV_r4 ) + cubeUV_m1;
		} else if ( roughness >= cubeUV_r5 ) {
			mip = ( cubeUV_r4 - roughness ) * ( cubeUV_m5 - cubeUV_m4 ) / ( cubeUV_r4 - cubeUV_r5 ) + cubeUV_m4;
		} else if ( roughness >= cubeUV_r6 ) {
			mip = ( cubeUV_r5 - roughness ) * ( cubeUV_m6 - cubeUV_m5 ) / ( cubeUV_r5 - cubeUV_r6 ) + cubeUV_m5;
		} else {
			mip = - 2.0 * log2( 1.16 * roughness );		}
		return mip;
	}
	vec4 textureCubeUV( sampler2D envMap, vec3 sampleDir, float roughness ) {
		float mip = clamp( roughnessToMip( roughness ), cubeUV_m0, CUBEUV_MAX_MIP );
		float mipF = fract( mip );
		float mipInt = floor( mip );
		vec3 color0 = bilinearCubeUV( envMap, sampleDir, mipInt );
		if ( mipF == 0.0 ) {
			return vec4( color0, 1.0 );
		} else {
			vec3 color1 = bilinearCubeUV( envMap, sampleDir, mipInt + 1.0 );
			return vec4( mix( color0, color1, mipF ), 1.0 );
		}
	}
#endif`,wm=`vec3 transformedNormal = objectNormal;
#ifdef USE_TANGENT
	vec3 transformedTangent = objectTangent;
#endif
#ifdef USE_BATCHING
	mat3 bm = mat3( batchingMatrix );
	transformedNormal /= vec3( dot( bm[ 0 ], bm[ 0 ] ), dot( bm[ 1 ], bm[ 1 ] ), dot( bm[ 2 ], bm[ 2 ] ) );
	transformedNormal = bm * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = bm * transformedTangent;
	#endif
#endif
#ifdef USE_INSTANCING
	mat3 im = mat3( instanceMatrix );
	transformedNormal /= vec3( dot( im[ 0 ], im[ 0 ] ), dot( im[ 1 ], im[ 1 ] ), dot( im[ 2 ], im[ 2 ] ) );
	transformedNormal = im * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = im * transformedTangent;
	#endif
#endif
transformedNormal = normalMatrix * transformedNormal;
#ifdef FLIP_SIDED
	transformedNormal = - transformedNormal;
#endif
#ifdef USE_TANGENT
	transformedTangent = ( modelViewMatrix * vec4( transformedTangent, 0.0 ) ).xyz;
	#ifdef FLIP_SIDED
		transformedTangent = - transformedTangent;
	#endif
#endif`,Am=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Em=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Tm=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,Cm=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Rm="gl_FragColor = linearToOutputTexel( gl_FragColor );",Pm=`
const mat3 LINEAR_SRGB_TO_LINEAR_DISPLAY_P3 = mat3(
	vec3( 0.8224621, 0.177538, 0.0 ),
	vec3( 0.0331941, 0.9668058, 0.0 ),
	vec3( 0.0170827, 0.0723974, 0.9105199 )
);
const mat3 LINEAR_DISPLAY_P3_TO_LINEAR_SRGB = mat3(
	vec3( 1.2249401, - 0.2249404, 0.0 ),
	vec3( - 0.0420569, 1.0420571, 0.0 ),
	vec3( - 0.0196376, - 0.0786361, 1.0982735 )
);
vec4 LinearSRGBToLinearDisplayP3( in vec4 value ) {
	return vec4( value.rgb * LINEAR_SRGB_TO_LINEAR_DISPLAY_P3, value.a );
}
vec4 LinearDisplayP3ToLinearSRGB( in vec4 value ) {
	return vec4( value.rgb * LINEAR_DISPLAY_P3_TO_LINEAR_SRGB, value.a );
}
vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Im=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vec3 cameraToFrag;
		if ( isOrthographic ) {
			cameraToFrag = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToFrag = normalize( vWorldPosition - cameraPosition );
		}
		vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vec3 reflectVec = reflect( cameraToFrag, worldNormal );
		#else
			vec3 reflectVec = refract( cameraToFrag, worldNormal, refractionRatio );
		#endif
	#else
		vec3 reflectVec = vReflect;
	#endif
	#ifdef ENVMAP_TYPE_CUBE
		vec4 envColor = textureCube( envMap, envMapRotation * vec3( flipEnvMap * reflectVec.x, reflectVec.yz ) );
	#else
		vec4 envColor = vec4( 0.0 );
	#endif
	#ifdef ENVMAP_BLENDING_MULTIPLY
		outgoingLight = mix( outgoingLight, outgoingLight * envColor.xyz, specularStrength * reflectivity );
	#elif defined( ENVMAP_BLENDING_MIX )
		outgoingLight = mix( outgoingLight, envColor.xyz, specularStrength * reflectivity );
	#elif defined( ENVMAP_BLENDING_ADD )
		outgoingLight += envColor.xyz * specularStrength * reflectivity;
	#endif
#endif`,Lm=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform float flipEnvMap;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
	
#endif`,Dm=`#ifdef USE_ENVMAP
	uniform float reflectivity;
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		varying vec3 vWorldPosition;
		uniform float refractionRatio;
	#else
		varying vec3 vReflect;
	#endif
#endif`,Um=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Nm=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vWorldPosition = worldPosition.xyz;
	#else
		vec3 cameraToVertex;
		if ( isOrthographic ) {
			cameraToVertex = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToVertex = normalize( worldPosition.xyz - cameraPosition );
		}
		vec3 worldNormal = inverseTransformDirection( transformedNormal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vReflect = reflect( cameraToVertex, worldNormal );
		#else
			vReflect = refract( cameraToVertex, worldNormal, refractionRatio );
		#endif
	#endif
#endif`,Om=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Fm=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,Bm=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,zm=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,km=`#ifdef USE_GRADIENTMAP
	uniform sampler2D gradientMap;
#endif
vec3 getGradientIrradiance( vec3 normal, vec3 lightDirection ) {
	float dotNL = dot( normal, lightDirection );
	vec2 coord = vec2( dotNL * 0.5 + 0.5, 0.0 );
	#ifdef USE_GRADIENTMAP
		return vec3( texture2D( gradientMap, coord ).r );
	#else
		vec2 fw = fwidth( coord ) * 0.5;
		return mix( vec3( 0.7 ), vec3( 1.0 ), smoothstep( 0.7 - fw.x, 0.7 + fw.x, coord.x ) );
	#endif
}`,Hm=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Vm=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Gm=`varying vec3 vViewPosition;
struct LambertMaterial {
	vec3 diffuseColor;
	float specularStrength;
};
void RE_Direct_Lambert( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Lambert( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Lambert
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,Wm=`uniform bool receiveShadow;
uniform vec3 ambientLightColor;
#if defined( USE_LIGHT_PROBES )
	uniform vec3 lightProbe[ 9 ];
#endif
vec3 shGetIrradianceAt( in vec3 normal, in vec3 shCoefficients[ 9 ] ) {
	float x = normal.x, y = normal.y, z = normal.z;
	vec3 result = shCoefficients[ 0 ] * 0.886227;
	result += shCoefficients[ 1 ] * 2.0 * 0.511664 * y;
	result += shCoefficients[ 2 ] * 2.0 * 0.511664 * z;
	result += shCoefficients[ 3 ] * 2.0 * 0.511664 * x;
	result += shCoefficients[ 4 ] * 2.0 * 0.429043 * x * y;
	result += shCoefficients[ 5 ] * 2.0 * 0.429043 * y * z;
	result += shCoefficients[ 6 ] * ( 0.743125 * z * z - 0.247708 );
	result += shCoefficients[ 7 ] * 2.0 * 0.429043 * x * z;
	result += shCoefficients[ 8 ] * 0.429043 * ( x * x - y * y );
	return result;
}
vec3 getLightProbeIrradiance( const in vec3 lightProbe[ 9 ], const in vec3 normal ) {
	vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
	vec3 irradiance = shGetIrradianceAt( worldNormal, lightProbe );
	return irradiance;
}
vec3 getAmbientLightIrradiance( const in vec3 ambientLightColor ) {
	vec3 irradiance = ambientLightColor;
	return irradiance;
}
float getDistanceAttenuation( const in float lightDistance, const in float cutoffDistance, const in float decayExponent ) {
	float distanceFalloff = 1.0 / max( pow( lightDistance, decayExponent ), 0.01 );
	if ( cutoffDistance > 0.0 ) {
		distanceFalloff *= pow2( saturate( 1.0 - pow4( lightDistance / cutoffDistance ) ) );
	}
	return distanceFalloff;
}
float getSpotAttenuation( const in float coneCosine, const in float penumbraCosine, const in float angleCosine ) {
	return smoothstep( coneCosine, penumbraCosine, angleCosine );
}
#if NUM_DIR_LIGHTS > 0
	struct DirectionalLight {
		vec3 direction;
		vec3 color;
	};
	uniform DirectionalLight directionalLights[ NUM_DIR_LIGHTS ];
	void getDirectionalLightInfo( const in DirectionalLight directionalLight, out IncidentLight light ) {
		light.color = directionalLight.color;
		light.direction = directionalLight.direction;
		light.visible = true;
	}
#endif
#if NUM_POINT_LIGHTS > 0
	struct PointLight {
		vec3 position;
		vec3 color;
		float distance;
		float decay;
	};
	uniform PointLight pointLights[ NUM_POINT_LIGHTS ];
	void getPointLightInfo( const in PointLight pointLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = pointLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float lightDistance = length( lVector );
		light.color = pointLight.color;
		light.color *= getDistanceAttenuation( lightDistance, pointLight.distance, pointLight.decay );
		light.visible = ( light.color != vec3( 0.0 ) );
	}
#endif
#if NUM_SPOT_LIGHTS > 0
	struct SpotLight {
		vec3 position;
		vec3 direction;
		vec3 color;
		float distance;
		float decay;
		float coneCos;
		float penumbraCos;
	};
	uniform SpotLight spotLights[ NUM_SPOT_LIGHTS ];
	void getSpotLightInfo( const in SpotLight spotLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = spotLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float angleCos = dot( light.direction, spotLight.direction );
		float spotAttenuation = getSpotAttenuation( spotLight.coneCos, spotLight.penumbraCos, angleCos );
		if ( spotAttenuation > 0.0 ) {
			float lightDistance = length( lVector );
			light.color = spotLight.color * spotAttenuation;
			light.color *= getDistanceAttenuation( lightDistance, spotLight.distance, spotLight.decay );
			light.visible = ( light.color != vec3( 0.0 ) );
		} else {
			light.color = vec3( 0.0 );
			light.visible = false;
		}
	}
#endif
#if NUM_RECT_AREA_LIGHTS > 0
	struct RectAreaLight {
		vec3 color;
		vec3 position;
		vec3 halfWidth;
		vec3 halfHeight;
	};
	uniform sampler2D ltc_1;	uniform sampler2D ltc_2;
	uniform RectAreaLight rectAreaLights[ NUM_RECT_AREA_LIGHTS ];
#endif
#if NUM_HEMI_LIGHTS > 0
	struct HemisphereLight {
		vec3 direction;
		vec3 skyColor;
		vec3 groundColor;
	};
	uniform HemisphereLight hemisphereLights[ NUM_HEMI_LIGHTS ];
	vec3 getHemisphereLightIrradiance( const in HemisphereLight hemiLight, const in vec3 normal ) {
		float dotNL = dot( normal, hemiLight.direction );
		float hemiDiffuseWeight = 0.5 * dotNL + 0.5;
		vec3 irradiance = mix( hemiLight.groundColor, hemiLight.skyColor, hemiDiffuseWeight );
		return irradiance;
	}
#endif`,Xm=`#ifdef USE_ENVMAP
	vec3 getIBLIrradiance( const in vec3 normal ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * worldNormal, 1.0 );
			return PI * envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	vec3 getIBLRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 reflectVec = reflect( - viewDir, normal );
			reflectVec = normalize( mix( reflectVec, normal, roughness * roughness) );
			reflectVec = inverseTransformDirection( reflectVec, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * reflectVec, roughness );
			return envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	#ifdef USE_ANISOTROPY
		vec3 getIBLAnisotropyRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness, const in vec3 bitangent, const in float anisotropy ) {
			#ifdef ENVMAP_TYPE_CUBE_UV
				vec3 bentNormal = cross( bitangent, viewDir );
				bentNormal = normalize( cross( bentNormal, bitangent ) );
				bentNormal = normalize( mix( bentNormal, normal, pow2( pow2( 1.0 - anisotropy * ( 1.0 - roughness ) ) ) ) );
				return getIBLRadiance( viewDir, bentNormal, roughness );
			#else
				return vec3( 0.0 );
			#endif
		}
	#endif
#endif`,qm=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,Ym=`varying vec3 vViewPosition;
struct ToonMaterial {
	vec3 diffuseColor;
};
void RE_Direct_Toon( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	vec3 irradiance = getGradientIrradiance( geometryNormal, directLight.direction ) * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Toon( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Toon
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,Zm=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,jm=`varying vec3 vViewPosition;
struct BlinnPhongMaterial {
	vec3 diffuseColor;
	vec3 specularColor;
	float specularShininess;
	float specularStrength;
};
void RE_Direct_BlinnPhong( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
	reflectedLight.directSpecular += irradiance * BRDF_BlinnPhong( directLight.direction, geometryViewDir, geometryNormal, material.specularColor, material.specularShininess ) * material.specularStrength;
}
void RE_IndirectDiffuse_BlinnPhong( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_BlinnPhong
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,Km=`PhysicalMaterial material;
material.diffuseColor = diffuseColor.rgb * ( 1.0 - metalnessFactor );
vec3 dxy = max( abs( dFdx( nonPerturbedNormal ) ), abs( dFdy( nonPerturbedNormal ) ) );
float geometryRoughness = max( max( dxy.x, dxy.y ), dxy.z );
material.roughness = max( roughnessFactor, 0.0525 );material.roughness += geometryRoughness;
material.roughness = min( material.roughness, 1.0 );
#ifdef IOR
	material.ior = ior;
	#ifdef USE_SPECULAR
		float specularIntensityFactor = specularIntensity;
		vec3 specularColorFactor = specularColor;
		#ifdef USE_SPECULAR_COLORMAP
			specularColorFactor *= texture2D( specularColorMap, vSpecularColorMapUv ).rgb;
		#endif
		#ifdef USE_SPECULAR_INTENSITYMAP
			specularIntensityFactor *= texture2D( specularIntensityMap, vSpecularIntensityMapUv ).a;
		#endif
		material.specularF90 = mix( specularIntensityFactor, 1.0, metalnessFactor );
	#else
		float specularIntensityFactor = 1.0;
		vec3 specularColorFactor = vec3( 1.0 );
		material.specularF90 = 1.0;
	#endif
	material.specularColor = mix( min( pow2( ( material.ior - 1.0 ) / ( material.ior + 1.0 ) ) * specularColorFactor, vec3( 1.0 ) ) * specularIntensityFactor, diffuseColor.rgb, metalnessFactor );
#else
	material.specularColor = mix( vec3( 0.04 ), diffuseColor.rgb, metalnessFactor );
	material.specularF90 = 1.0;
#endif
#ifdef USE_CLEARCOAT
	material.clearcoat = clearcoat;
	material.clearcoatRoughness = clearcoatRoughness;
	material.clearcoatF0 = vec3( 0.04 );
	material.clearcoatF90 = 1.0;
	#ifdef USE_CLEARCOATMAP
		material.clearcoat *= texture2D( clearcoatMap, vClearcoatMapUv ).x;
	#endif
	#ifdef USE_CLEARCOAT_ROUGHNESSMAP
		material.clearcoatRoughness *= texture2D( clearcoatRoughnessMap, vClearcoatRoughnessMapUv ).y;
	#endif
	material.clearcoat = saturate( material.clearcoat );	material.clearcoatRoughness = max( material.clearcoatRoughness, 0.0525 );
	material.clearcoatRoughness += geometryRoughness;
	material.clearcoatRoughness = min( material.clearcoatRoughness, 1.0 );
#endif
#ifdef USE_DISPERSION
	material.dispersion = dispersion;
#endif
#ifdef USE_IRIDESCENCE
	material.iridescence = iridescence;
	material.iridescenceIOR = iridescenceIOR;
	#ifdef USE_IRIDESCENCEMAP
		material.iridescence *= texture2D( iridescenceMap, vIridescenceMapUv ).r;
	#endif
	#ifdef USE_IRIDESCENCE_THICKNESSMAP
		material.iridescenceThickness = (iridescenceThicknessMaximum - iridescenceThicknessMinimum) * texture2D( iridescenceThicknessMap, vIridescenceThicknessMapUv ).g + iridescenceThicknessMinimum;
	#else
		material.iridescenceThickness = iridescenceThicknessMaximum;
	#endif
#endif
#ifdef USE_SHEEN
	material.sheenColor = sheenColor;
	#ifdef USE_SHEEN_COLORMAP
		material.sheenColor *= texture2D( sheenColorMap, vSheenColorMapUv ).rgb;
	#endif
	material.sheenRoughness = clamp( sheenRoughness, 0.07, 1.0 );
	#ifdef USE_SHEEN_ROUGHNESSMAP
		material.sheenRoughness *= texture2D( sheenRoughnessMap, vSheenRoughnessMapUv ).a;
	#endif
#endif
#ifdef USE_ANISOTROPY
	#ifdef USE_ANISOTROPYMAP
		mat2 anisotropyMat = mat2( anisotropyVector.x, anisotropyVector.y, - anisotropyVector.y, anisotropyVector.x );
		vec3 anisotropyPolar = texture2D( anisotropyMap, vAnisotropyMapUv ).rgb;
		vec2 anisotropyV = anisotropyMat * normalize( 2.0 * anisotropyPolar.rg - vec2( 1.0 ) ) * anisotropyPolar.b;
	#else
		vec2 anisotropyV = anisotropyVector;
	#endif
	material.anisotropy = length( anisotropyV );
	if( material.anisotropy == 0.0 ) {
		anisotropyV = vec2( 1.0, 0.0 );
	} else {
		anisotropyV /= material.anisotropy;
		material.anisotropy = saturate( material.anisotropy );
	}
	material.alphaT = mix( pow2( material.roughness ), 1.0, pow2( material.anisotropy ) );
	material.anisotropyT = tbn[ 0 ] * anisotropyV.x + tbn[ 1 ] * anisotropyV.y;
	material.anisotropyB = tbn[ 1 ] * anisotropyV.x - tbn[ 0 ] * anisotropyV.y;
#endif`,Jm=`struct PhysicalMaterial {
	vec3 diffuseColor;
	float roughness;
	vec3 specularColor;
	float specularF90;
	float dispersion;
	#ifdef USE_CLEARCOAT
		float clearcoat;
		float clearcoatRoughness;
		vec3 clearcoatF0;
		float clearcoatF90;
	#endif
	#ifdef USE_IRIDESCENCE
		float iridescence;
		float iridescenceIOR;
		float iridescenceThickness;
		vec3 iridescenceFresnel;
		vec3 iridescenceF0;
	#endif
	#ifdef USE_SHEEN
		vec3 sheenColor;
		float sheenRoughness;
	#endif
	#ifdef IOR
		float ior;
	#endif
	#ifdef USE_TRANSMISSION
		float transmission;
		float transmissionAlpha;
		float thickness;
		float attenuationDistance;
		vec3 attenuationColor;
	#endif
	#ifdef USE_ANISOTROPY
		float anisotropy;
		float alphaT;
		vec3 anisotropyT;
		vec3 anisotropyB;
	#endif
};
vec3 clearcoatSpecularDirect = vec3( 0.0 );
vec3 clearcoatSpecularIndirect = vec3( 0.0 );
vec3 sheenSpecularDirect = vec3( 0.0 );
vec3 sheenSpecularIndirect = vec3(0.0 );
vec3 Schlick_to_F0( const in vec3 f, const in float f90, const in float dotVH ) {
    float x = clamp( 1.0 - dotVH, 0.0, 1.0 );
    float x2 = x * x;
    float x5 = clamp( x * x2 * x2, 0.0, 0.9999 );
    return ( f - vec3( f90 ) * x5 ) / ( 1.0 - x5 );
}
float V_GGX_SmithCorrelated( const in float alpha, const in float dotNL, const in float dotNV ) {
	float a2 = pow2( alpha );
	float gv = dotNL * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNV ) );
	float gl = dotNV * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNL ) );
	return 0.5 / max( gv + gl, EPSILON );
}
float D_GGX( const in float alpha, const in float dotNH ) {
	float a2 = pow2( alpha );
	float denom = pow2( dotNH ) * ( a2 - 1.0 ) + 1.0;
	return RECIPROCAL_PI * a2 / pow2( denom );
}
#ifdef USE_ANISOTROPY
	float V_GGX_SmithCorrelated_Anisotropic( const in float alphaT, const in float alphaB, const in float dotTV, const in float dotBV, const in float dotTL, const in float dotBL, const in float dotNV, const in float dotNL ) {
		float gv = dotNL * length( vec3( alphaT * dotTV, alphaB * dotBV, dotNV ) );
		float gl = dotNV * length( vec3( alphaT * dotTL, alphaB * dotBL, dotNL ) );
		float v = 0.5 / ( gv + gl );
		return saturate(v);
	}
	float D_GGX_Anisotropic( const in float alphaT, const in float alphaB, const in float dotNH, const in float dotTH, const in float dotBH ) {
		float a2 = alphaT * alphaB;
		highp vec3 v = vec3( alphaB * dotTH, alphaT * dotBH, a2 * dotNH );
		highp float v2 = dot( v, v );
		float w2 = a2 / v2;
		return RECIPROCAL_PI * a2 * pow2 ( w2 );
	}
#endif
#ifdef USE_CLEARCOAT
	vec3 BRDF_GGX_Clearcoat( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material) {
		vec3 f0 = material.clearcoatF0;
		float f90 = material.clearcoatF90;
		float roughness = material.clearcoatRoughness;
		float alpha = pow2( roughness );
		vec3 halfDir = normalize( lightDir + viewDir );
		float dotNL = saturate( dot( normal, lightDir ) );
		float dotNV = saturate( dot( normal, viewDir ) );
		float dotNH = saturate( dot( normal, halfDir ) );
		float dotVH = saturate( dot( viewDir, halfDir ) );
		vec3 F = F_Schlick( f0, f90, dotVH );
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
		return F * ( V * D );
	}
#endif
vec3 BRDF_GGX( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material ) {
	vec3 f0 = material.specularColor;
	float f90 = material.specularF90;
	float roughness = material.roughness;
	float alpha = pow2( roughness );
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( f0, f90, dotVH );
	#ifdef USE_IRIDESCENCE
		F = mix( F, material.iridescenceFresnel, material.iridescence );
	#endif
	#ifdef USE_ANISOTROPY
		float dotTL = dot( material.anisotropyT, lightDir );
		float dotTV = dot( material.anisotropyT, viewDir );
		float dotTH = dot( material.anisotropyT, halfDir );
		float dotBL = dot( material.anisotropyB, lightDir );
		float dotBV = dot( material.anisotropyB, viewDir );
		float dotBH = dot( material.anisotropyB, halfDir );
		float V = V_GGX_SmithCorrelated_Anisotropic( material.alphaT, alpha, dotTV, dotBV, dotTL, dotBL, dotNV, dotNL );
		float D = D_GGX_Anisotropic( material.alphaT, alpha, dotNH, dotTH, dotBH );
	#else
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
	#endif
	return F * ( V * D );
}
vec2 LTC_Uv( const in vec3 N, const in vec3 V, const in float roughness ) {
	const float LUT_SIZE = 64.0;
	const float LUT_SCALE = ( LUT_SIZE - 1.0 ) / LUT_SIZE;
	const float LUT_BIAS = 0.5 / LUT_SIZE;
	float dotNV = saturate( dot( N, V ) );
	vec2 uv = vec2( roughness, sqrt( 1.0 - dotNV ) );
	uv = uv * LUT_SCALE + LUT_BIAS;
	return uv;
}
float LTC_ClippedSphereFormFactor( const in vec3 f ) {
	float l = length( f );
	return max( ( l * l + f.z ) / ( l + 1.0 ), 0.0 );
}
vec3 LTC_EdgeVectorFormFactor( const in vec3 v1, const in vec3 v2 ) {
	float x = dot( v1, v2 );
	float y = abs( x );
	float a = 0.8543985 + ( 0.4965155 + 0.0145206 * y ) * y;
	float b = 3.4175940 + ( 4.1616724 + y ) * y;
	float v = a / b;
	float theta_sintheta = ( x > 0.0 ) ? v : 0.5 * inversesqrt( max( 1.0 - x * x, 1e-7 ) ) - v;
	return cross( v1, v2 ) * theta_sintheta;
}
vec3 LTC_Evaluate( const in vec3 N, const in vec3 V, const in vec3 P, const in mat3 mInv, const in vec3 rectCoords[ 4 ] ) {
	vec3 v1 = rectCoords[ 1 ] - rectCoords[ 0 ];
	vec3 v2 = rectCoords[ 3 ] - rectCoords[ 0 ];
	vec3 lightNormal = cross( v1, v2 );
	if( dot( lightNormal, P - rectCoords[ 0 ] ) < 0.0 ) return vec3( 0.0 );
	vec3 T1, T2;
	T1 = normalize( V - N * dot( V, N ) );
	T2 = - cross( N, T1 );
	mat3 mat = mInv * transposeMat3( mat3( T1, T2, N ) );
	vec3 coords[ 4 ];
	coords[ 0 ] = mat * ( rectCoords[ 0 ] - P );
	coords[ 1 ] = mat * ( rectCoords[ 1 ] - P );
	coords[ 2 ] = mat * ( rectCoords[ 2 ] - P );
	coords[ 3 ] = mat * ( rectCoords[ 3 ] - P );
	coords[ 0 ] = normalize( coords[ 0 ] );
	coords[ 1 ] = normalize( coords[ 1 ] );
	coords[ 2 ] = normalize( coords[ 2 ] );
	coords[ 3 ] = normalize( coords[ 3 ] );
	vec3 vectorFormFactor = vec3( 0.0 );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 0 ], coords[ 1 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 1 ], coords[ 2 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 2 ], coords[ 3 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 3 ], coords[ 0 ] );
	float result = LTC_ClippedSphereFormFactor( vectorFormFactor );
	return vec3( result );
}
#if defined( USE_SHEEN )
float D_Charlie( float roughness, float dotNH ) {
	float alpha = pow2( roughness );
	float invAlpha = 1.0 / alpha;
	float cos2h = dotNH * dotNH;
	float sin2h = max( 1.0 - cos2h, 0.0078125 );
	return ( 2.0 + invAlpha ) * pow( sin2h, invAlpha * 0.5 ) / ( 2.0 * PI );
}
float V_Neubelt( float dotNV, float dotNL ) {
	return saturate( 1.0 / ( 4.0 * ( dotNL + dotNV - dotNL * dotNV ) ) );
}
vec3 BRDF_Sheen( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, vec3 sheenColor, const in float sheenRoughness ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float D = D_Charlie( sheenRoughness, dotNH );
	float V = V_Neubelt( dotNV, dotNL );
	return sheenColor * ( D * V );
}
#endif
float IBLSheenBRDF( const in vec3 normal, const in vec3 viewDir, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	float r2 = roughness * roughness;
	float a = roughness < 0.25 ? -339.2 * r2 + 161.4 * roughness - 25.9 : -8.48 * r2 + 14.3 * roughness - 9.95;
	float b = roughness < 0.25 ? 44.0 * r2 - 23.7 * roughness + 3.26 : 1.97 * r2 - 3.27 * roughness + 0.72;
	float DG = exp( a * dotNV + b ) + ( roughness < 0.25 ? 0.0 : 0.1 * ( roughness - 0.25 ) );
	return saturate( DG * RECIPROCAL_PI );
}
vec2 DFGApprox( const in vec3 normal, const in vec3 viewDir, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	const vec4 c0 = vec4( - 1, - 0.0275, - 0.572, 0.022 );
	const vec4 c1 = vec4( 1, 0.0425, 1.04, - 0.04 );
	vec4 r = roughness * c0 + c1;
	float a004 = min( r.x * r.x, exp2( - 9.28 * dotNV ) ) * r.x + r.y;
	vec2 fab = vec2( - 1.04, 1.04 ) * a004 + r.zw;
	return fab;
}
vec3 EnvironmentBRDF( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness ) {
	vec2 fab = DFGApprox( normal, viewDir, roughness );
	return specularColor * fab.x + specularF90 * fab.y;
}
#ifdef USE_IRIDESCENCE
void computeMultiscatteringIridescence( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float iridescence, const in vec3 iridescenceF0, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#else
void computeMultiscattering( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#endif
	vec2 fab = DFGApprox( normal, viewDir, roughness );
	#ifdef USE_IRIDESCENCE
		vec3 Fr = mix( specularColor, iridescenceF0, iridescence );
	#else
		vec3 Fr = specularColor;
	#endif
	vec3 FssEss = Fr * fab.x + specularF90 * fab.y;
	float Ess = fab.x + fab.y;
	float Ems = 1.0 - Ess;
	vec3 Favg = Fr + ( 1.0 - Fr ) * 0.047619;	vec3 Fms = FssEss * Favg / ( 1.0 - Ems * Favg );
	singleScatter += FssEss;
	multiScatter += Fms * Ems;
}
#if NUM_RECT_AREA_LIGHTS > 0
	void RE_Direct_RectArea_Physical( const in RectAreaLight rectAreaLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
		vec3 normal = geometryNormal;
		vec3 viewDir = geometryViewDir;
		vec3 position = geometryPosition;
		vec3 lightPos = rectAreaLight.position;
		vec3 halfWidth = rectAreaLight.halfWidth;
		vec3 halfHeight = rectAreaLight.halfHeight;
		vec3 lightColor = rectAreaLight.color;
		float roughness = material.roughness;
		vec3 rectCoords[ 4 ];
		rectCoords[ 0 ] = lightPos + halfWidth - halfHeight;		rectCoords[ 1 ] = lightPos - halfWidth - halfHeight;
		rectCoords[ 2 ] = lightPos - halfWidth + halfHeight;
		rectCoords[ 3 ] = lightPos + halfWidth + halfHeight;
		vec2 uv = LTC_Uv( normal, viewDir, roughness );
		vec4 t1 = texture2D( ltc_1, uv );
		vec4 t2 = texture2D( ltc_2, uv );
		mat3 mInv = mat3(
			vec3( t1.x, 0, t1.y ),
			vec3(    0, 1,    0 ),
			vec3( t1.z, 0, t1.w )
		);
		vec3 fresnel = ( material.specularColor * t2.x + ( vec3( 1.0 ) - material.specularColor ) * t2.y );
		reflectedLight.directSpecular += lightColor * fresnel * LTC_Evaluate( normal, viewDir, position, mInv, rectCoords );
		reflectedLight.directDiffuse += lightColor * material.diffuseColor * LTC_Evaluate( normal, viewDir, position, mat3( 1.0 ), rectCoords );
	}
#endif
void RE_Direct_Physical( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	#ifdef USE_CLEARCOAT
		float dotNLcc = saturate( dot( geometryClearcoatNormal, directLight.direction ) );
		vec3 ccIrradiance = dotNLcc * directLight.color;
		clearcoatSpecularDirect += ccIrradiance * BRDF_GGX_Clearcoat( directLight.direction, geometryViewDir, geometryClearcoatNormal, material );
	#endif
	#ifdef USE_SHEEN
		sheenSpecularDirect += irradiance * BRDF_Sheen( directLight.direction, geometryViewDir, geometryNormal, material.sheenColor, material.sheenRoughness );
	#endif
	reflectedLight.directSpecular += irradiance * BRDF_GGX( directLight.direction, geometryViewDir, geometryNormal, material );
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Physical( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectSpecular_Physical( const in vec3 radiance, const in vec3 irradiance, const in vec3 clearcoatRadiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight) {
	#ifdef USE_CLEARCOAT
		clearcoatSpecularIndirect += clearcoatRadiance * EnvironmentBRDF( geometryClearcoatNormal, geometryViewDir, material.clearcoatF0, material.clearcoatF90, material.clearcoatRoughness );
	#endif
	#ifdef USE_SHEEN
		sheenSpecularIndirect += irradiance * material.sheenColor * IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
	#endif
	vec3 singleScattering = vec3( 0.0 );
	vec3 multiScattering = vec3( 0.0 );
	vec3 cosineWeightedIrradiance = irradiance * RECIPROCAL_PI;
	#ifdef USE_IRIDESCENCE
		computeMultiscatteringIridescence( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.iridescence, material.iridescenceFresnel, material.roughness, singleScattering, multiScattering );
	#else
		computeMultiscattering( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.roughness, singleScattering, multiScattering );
	#endif
	vec3 totalScattering = singleScattering + multiScattering;
	vec3 diffuse = material.diffuseColor * ( 1.0 - max( max( totalScattering.r, totalScattering.g ), totalScattering.b ) );
	reflectedLight.indirectSpecular += radiance * singleScattering;
	reflectedLight.indirectSpecular += multiScattering * cosineWeightedIrradiance;
	reflectedLight.indirectDiffuse += diffuse * cosineWeightedIrradiance;
}
#define RE_Direct				RE_Direct_Physical
#define RE_Direct_RectArea		RE_Direct_RectArea_Physical
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Physical
#define RE_IndirectSpecular		RE_IndirectSpecular_Physical
float computeSpecularOcclusion( const in float dotNV, const in float ambientOcclusion, const in float roughness ) {
	return saturate( pow( dotNV + ambientOcclusion, exp2( - 16.0 * roughness - 1.0 ) ) - 1.0 + ambientOcclusion );
}`,Qm=`
vec3 geometryPosition = - vViewPosition;
vec3 geometryNormal = normal;
vec3 geometryViewDir = ( isOrthographic ) ? vec3( 0, 0, 1 ) : normalize( vViewPosition );
vec3 geometryClearcoatNormal = vec3( 0.0 );
#ifdef USE_CLEARCOAT
	geometryClearcoatNormal = clearcoatNormal;
#endif
#ifdef USE_IRIDESCENCE
	float dotNVi = saturate( dot( normal, geometryViewDir ) );
	if ( material.iridescenceThickness == 0.0 ) {
		material.iridescence = 0.0;
	} else {
		material.iridescence = saturate( material.iridescence );
	}
	if ( material.iridescence > 0.0 ) {
		material.iridescenceFresnel = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.specularColor );
		material.iridescenceF0 = Schlick_to_F0( material.iridescenceFresnel, 1.0, dotNVi );
	}
#endif
IncidentLight directLight;
#if ( NUM_POINT_LIGHTS > 0 ) && defined( RE_Direct )
	PointLight pointLight;
	#if defined( USE_SHADOWMAP ) && NUM_POINT_LIGHT_SHADOWS > 0
	PointLightShadow pointLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHTS; i ++ ) {
		pointLight = pointLights[ i ];
		getPointLightInfo( pointLight, geometryPosition, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_POINT_LIGHT_SHADOWS )
		pointLightShadow = pointLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getPointShadow( pointShadowMap[ i ], pointLightShadow.shadowMapSize, pointLightShadow.shadowIntensity, pointLightShadow.shadowBias, pointLightShadow.shadowRadius, vPointShadowCoord[ i ], pointLightShadow.shadowCameraNear, pointLightShadow.shadowCameraFar ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_SPOT_LIGHTS > 0 ) && defined( RE_Direct )
	SpotLight spotLight;
	vec4 spotColor;
	vec3 spotLightCoord;
	bool inSpotLightMap;
	#if defined( USE_SHADOWMAP ) && NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHTS; i ++ ) {
		spotLight = spotLights[ i ];
		getSpotLightInfo( spotLight, geometryPosition, directLight );
		#if ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#define SPOT_LIGHT_MAP_INDEX UNROLLED_LOOP_INDEX
		#elif ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		#define SPOT_LIGHT_MAP_INDEX NUM_SPOT_LIGHT_MAPS
		#else
		#define SPOT_LIGHT_MAP_INDEX ( UNROLLED_LOOP_INDEX - NUM_SPOT_LIGHT_SHADOWS + NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#endif
		#if ( SPOT_LIGHT_MAP_INDEX < NUM_SPOT_LIGHT_MAPS )
			spotLightCoord = vSpotLightCoord[ i ].xyz / vSpotLightCoord[ i ].w;
			inSpotLightMap = all( lessThan( abs( spotLightCoord * 2. - 1. ), vec3( 1.0 ) ) );
			spotColor = texture2D( spotLightMap[ SPOT_LIGHT_MAP_INDEX ], spotLightCoord.xy );
			directLight.color = inSpotLightMap ? directLight.color * spotColor.rgb : directLight.color;
		#endif
		#undef SPOT_LIGHT_MAP_INDEX
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		spotLightShadow = spotLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( spotShadowMap[ i ], spotLightShadow.shadowMapSize, spotLightShadow.shadowIntensity, spotLightShadow.shadowBias, spotLightShadow.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_DIR_LIGHTS > 0 ) && defined( RE_Direct )
	DirectionalLight directionalLight;
	#if defined( USE_SHADOWMAP ) && NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHTS; i ++ ) {
		directionalLight = directionalLights[ i ];
		getDirectionalLightInfo( directionalLight, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_DIR_LIGHT_SHADOWS )
		directionalLightShadow = directionalLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( directionalShadowMap[ i ], directionalLightShadow.shadowMapSize, directionalLightShadow.shadowIntensity, directionalLightShadow.shadowBias, directionalLightShadow.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_RECT_AREA_LIGHTS > 0 ) && defined( RE_Direct_RectArea )
	RectAreaLight rectAreaLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_RECT_AREA_LIGHTS; i ++ ) {
		rectAreaLight = rectAreaLights[ i ];
		RE_Direct_RectArea( rectAreaLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if defined( RE_IndirectDiffuse )
	vec3 iblIrradiance = vec3( 0.0 );
	vec3 irradiance = getAmbientLightIrradiance( ambientLightColor );
	#if defined( USE_LIGHT_PROBES )
		irradiance += getLightProbeIrradiance( lightProbe, geometryNormal );
	#endif
	#if ( NUM_HEMI_LIGHTS > 0 )
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_HEMI_LIGHTS; i ++ ) {
			irradiance += getHemisphereLightIrradiance( hemisphereLights[ i ], geometryNormal );
		}
		#pragma unroll_loop_end
	#endif
#endif
#if defined( RE_IndirectSpecular )
	vec3 radiance = vec3( 0.0 );
	vec3 clearcoatRadiance = vec3( 0.0 );
#endif`,$m=`#if defined( RE_IndirectDiffuse )
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		vec3 lightMapIrradiance = lightMapTexel.rgb * lightMapIntensity;
		irradiance += lightMapIrradiance;
	#endif
	#if defined( USE_ENVMAP ) && defined( STANDARD ) && defined( ENVMAP_TYPE_CUBE_UV )
		iblIrradiance += getIBLIrradiance( geometryNormal );
	#endif
#endif
#if defined( USE_ENVMAP ) && defined( RE_IndirectSpecular )
	#ifdef USE_ANISOTROPY
		radiance += getIBLAnisotropyRadiance( geometryViewDir, geometryNormal, material.roughness, material.anisotropyB, material.anisotropy );
	#else
		radiance += getIBLRadiance( geometryViewDir, geometryNormal, material.roughness );
	#endif
	#ifdef USE_CLEARCOAT
		clearcoatRadiance += getIBLRadiance( geometryViewDir, geometryClearcoatNormal, material.clearcoatRoughness );
	#endif
#endif`,tg=`#if defined( RE_IndirectDiffuse )
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,eg=`#if defined( USE_LOGDEPTHBUF )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,ng=`#if defined( USE_LOGDEPTHBUF )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,ig=`#ifdef USE_LOGDEPTHBUF
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,sg=`#ifdef USE_LOGDEPTHBUF
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,rg=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = vec4( mix( pow( sampledDiffuseColor.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), sampledDiffuseColor.rgb * 0.0773993808, vec3( lessThanEqual( sampledDiffuseColor.rgb, vec3( 0.04045 ) ) ) ), sampledDiffuseColor.w );
	
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,og=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,ag=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
	#if defined( USE_POINTS_UV )
		vec2 uv = vUv;
	#else
		vec2 uv = ( uvTransform * vec3( gl_PointCoord.x, 1.0 - gl_PointCoord.y, 1 ) ).xy;
	#endif
#endif
#ifdef USE_MAP
	diffuseColor *= texture2D( map, uv );
#endif
#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, uv ).g;
#endif`,cg=`#if defined( USE_POINTS_UV )
	varying vec2 vUv;
#else
	#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
		uniform mat3 uvTransform;
	#endif
#endif
#ifdef USE_MAP
	uniform sampler2D map;
#endif
#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,lg=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,hg=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,ug=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,dg=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,fg=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,pg=`#ifdef USE_MORPHTARGETS
	#ifndef USE_INSTANCING_MORPH
		uniform float morphTargetBaseInfluence;
		uniform float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	#endif
	uniform sampler2DArray morphTargetsTexture;
	uniform ivec2 morphTargetsTextureSize;
	vec4 getMorph( const in int vertexIndex, const in int morphTargetIndex, const in int offset ) {
		int texelIndex = vertexIndex * MORPHTARGETS_TEXTURE_STRIDE + offset;
		int y = texelIndex / morphTargetsTextureSize.x;
		int x = texelIndex - y * morphTargetsTextureSize.x;
		ivec3 morphUV = ivec3( x, y, morphTargetIndex );
		return texelFetch( morphTargetsTexture, morphUV, 0 );
	}
#endif`,mg=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,gg=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
#ifdef FLAT_SHADED
	vec3 fdx = dFdx( vViewPosition );
	vec3 fdy = dFdy( vViewPosition );
	vec3 normal = normalize( cross( fdx, fdy ) );
#else
	vec3 normal = normalize( vNormal );
	#ifdef DOUBLE_SIDED
		normal *= faceDirection;
	#endif
#endif
#if defined( USE_NORMALMAP_TANGENTSPACE ) || defined( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY )
	#ifdef USE_TANGENT
		mat3 tbn = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn = getTangentFrame( - vViewPosition, normal,
		#if defined( USE_NORMALMAP )
			vNormalMapUv
		#elif defined( USE_CLEARCOAT_NORMALMAP )
			vClearcoatNormalMapUv
		#else
			vUv
		#endif
		);
	#endif
	#if defined( DOUBLE_SIDED ) && ! defined( FLAT_SHADED )
		tbn[0] *= faceDirection;
		tbn[1] *= faceDirection;
	#endif
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	#ifdef USE_TANGENT
		mat3 tbn2 = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn2 = getTangentFrame( - vViewPosition, normal, vClearcoatNormalMapUv );
	#endif
	#if defined( DOUBLE_SIDED ) && ! defined( FLAT_SHADED )
		tbn2[0] *= faceDirection;
		tbn2[1] *= faceDirection;
	#endif
#endif
vec3 nonPerturbedNormal = normal;`,xg=`#ifdef USE_NORMALMAP_OBJECTSPACE
	normal = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	#ifdef FLIP_SIDED
		normal = - normal;
	#endif
	#ifdef DOUBLE_SIDED
		normal = normal * faceDirection;
	#endif
	normal = normalize( normalMatrix * normal );
#elif defined( USE_NORMALMAP_TANGENTSPACE )
	vec3 mapN = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	mapN.xy *= normalScale;
	normal = normalize( tbn * mapN );
#elif defined( USE_BUMPMAP )
	normal = perturbNormalArb( - vViewPosition, normal, dHdxy_fwd(), faceDirection );
#endif`,vg=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,_g=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,yg=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
	#endif
#endif`,Mg=`#ifdef USE_NORMALMAP
	uniform sampler2D normalMap;
	uniform vec2 normalScale;
#endif
#ifdef USE_NORMALMAP_OBJECTSPACE
	uniform mat3 normalMatrix;
#endif
#if ! defined ( USE_TANGENT ) && ( defined ( USE_NORMALMAP_TANGENTSPACE ) || defined ( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY ) )
	mat3 getTangentFrame( vec3 eye_pos, vec3 surf_norm, vec2 uv ) {
		vec3 q0 = dFdx( eye_pos.xyz );
		vec3 q1 = dFdy( eye_pos.xyz );
		vec2 st0 = dFdx( uv.st );
		vec2 st1 = dFdy( uv.st );
		vec3 N = surf_norm;
		vec3 q1perp = cross( q1, N );
		vec3 q0perp = cross( N, q0 );
		vec3 T = q1perp * st0.x + q0perp * st1.x;
		vec3 B = q1perp * st0.y + q0perp * st1.y;
		float det = max( dot( T, T ), dot( B, B ) );
		float scale = ( det == 0.0 ) ? 0.0 : inversesqrt( det );
		return mat3( T * scale, B * scale, N );
	}
#endif`,bg=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,Sg=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,wg=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,Ag=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Eg=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,Tg=`vec3 packNormalToRGB( const in vec3 normal ) {
	return normalize( normal ) * 0.5 + 0.5;
}
vec3 unpackRGBToNormal( const in vec3 rgb ) {
	return 2.0 * rgb.xyz - 1.0;
}
const float PackUpscale = 256. / 255.;const float UnpackDownscale = 255. / 256.;const float ShiftRight8 = 1. / 256.;
const float Inv255 = 1. / 255.;
const vec4 PackFactors = vec4( 1.0, 256.0, 256.0 * 256.0, 256.0 * 256.0 * 256.0 );
const vec2 UnpackFactors2 = vec2( UnpackDownscale, 1.0 / PackFactors.g );
const vec3 UnpackFactors3 = vec3( UnpackDownscale / PackFactors.rg, 1.0 / PackFactors.b );
const vec4 UnpackFactors4 = vec4( UnpackDownscale / PackFactors.rgb, 1.0 / PackFactors.a );
vec4 packDepthToRGBA( const in float v ) {
	if( v <= 0.0 )
		return vec4( 0., 0., 0., 0. );
	if( v >= 1.0 )
		return vec4( 1., 1., 1., 1. );
	float vuf;
	float af = modf( v * PackFactors.a, vuf );
	float bf = modf( vuf * ShiftRight8, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec4( vuf * Inv255, gf * PackUpscale, bf * PackUpscale, af );
}
vec3 packDepthToRGB( const in float v ) {
	if( v <= 0.0 )
		return vec3( 0., 0., 0. );
	if( v >= 1.0 )
		return vec3( 1., 1., 1. );
	float vuf;
	float bf = modf( v * PackFactors.b, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec3( vuf * Inv255, gf * PackUpscale, bf );
}
vec2 packDepthToRG( const in float v ) {
	if( v <= 0.0 )
		return vec2( 0., 0. );
	if( v >= 1.0 )
		return vec2( 1., 1. );
	float vuf;
	float gf = modf( v * 256., vuf );
	return vec2( vuf * Inv255, gf );
}
float unpackRGBAToDepth( const in vec4 v ) {
	return dot( v, UnpackFactors4 );
}
float unpackRGBToDepth( const in vec3 v ) {
	return dot( v, UnpackFactors3 );
}
float unpackRGToDepth( const in vec2 v ) {
	return v.r * UnpackFactors2.r + v.g * UnpackFactors2.g;
}
vec4 pack2HalfToRGBA( const in vec2 v ) {
	vec4 r = vec4( v.x, fract( v.x * 255.0 ), v.y, fract( v.y * 255.0 ) );
	return vec4( r.x - r.y / 255.0, r.y, r.z - r.w / 255.0, r.w );
}
vec2 unpackRGBATo2Half( const in vec4 v ) {
	return vec2( v.x + ( v.y / 255.0 ), v.z + ( v.w / 255.0 ) );
}
float viewZToOrthographicDepth( const in float viewZ, const in float near, const in float far ) {
	return ( viewZ + near ) / ( near - far );
}
float orthographicDepthToViewZ( const in float depth, const in float near, const in float far ) {
	return depth * ( near - far ) - near;
}
float viewZToPerspectiveDepth( const in float viewZ, const in float near, const in float far ) {
	return ( ( near + viewZ ) * far ) / ( ( far - near ) * viewZ );
}
float perspectiveDepthToViewZ( const in float depth, const in float near, const in float far ) {
	return ( near * far ) / ( ( far - near ) * depth - far );
}`,Cg=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,Rg=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Pg=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,Ig=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,Lg=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Dg=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,Ug=`#if NUM_SPOT_LIGHT_COORDS > 0
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#if NUM_SPOT_LIGHT_MAPS > 0
	uniform sampler2D spotLightMap[ NUM_SPOT_LIGHT_MAPS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
		uniform sampler2D directionalShadowMap[ NUM_DIR_LIGHT_SHADOWS ];
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		uniform sampler2D spotShadowMap[ NUM_SPOT_LIGHT_SHADOWS ];
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		uniform sampler2D pointShadowMap[ NUM_POINT_LIGHT_SHADOWS ];
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
	float texture2DCompare( sampler2D depths, vec2 uv, float compare ) {
		return step( compare, unpackRGBAToDepth( texture2D( depths, uv ) ) );
	}
	vec2 texture2DDistribution( sampler2D shadow, vec2 uv ) {
		return unpackRGBATo2Half( texture2D( shadow, uv ) );
	}
	float VSMShadow (sampler2D shadow, vec2 uv, float compare ){
		float occlusion = 1.0;
		vec2 distribution = texture2DDistribution( shadow, uv );
		float hard_shadow = step( compare , distribution.x );
		if (hard_shadow != 1.0 ) {
			float distance = compare - distribution.x ;
			float variance = max( 0.00000, distribution.y * distribution.y );
			float softness_probability = variance / (variance + distance * distance );			softness_probability = clamp( ( softness_probability - 0.3 ) / ( 0.95 - 0.3 ), 0.0, 1.0 );			occlusion = clamp( max( hard_shadow, softness_probability ), 0.0, 1.0 );
		}
		return occlusion;
	}
	float getShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord ) {
		float shadow = 1.0;
		shadowCoord.xyz /= shadowCoord.w;
		shadowCoord.z += shadowBias;
		bool inFrustum = shadowCoord.x >= 0.0 && shadowCoord.x <= 1.0 && shadowCoord.y >= 0.0 && shadowCoord.y <= 1.0;
		bool frustumTest = inFrustum && shadowCoord.z <= 1.0;
		if ( frustumTest ) {
		#if defined( SHADOWMAP_TYPE_PCF )
			vec2 texelSize = vec2( 1.0 ) / shadowMapSize;
			float dx0 = - texelSize.x * shadowRadius;
			float dy0 = - texelSize.y * shadowRadius;
			float dx1 = + texelSize.x * shadowRadius;
			float dy1 = + texelSize.y * shadowRadius;
			float dx2 = dx0 / 2.0;
			float dy2 = dy0 / 2.0;
			float dx3 = dx1 / 2.0;
			float dy3 = dy1 / 2.0;
			shadow = (
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy, shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, dy1 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy1 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, dy1 ), shadowCoord.z )
			) * ( 1.0 / 17.0 );
		#elif defined( SHADOWMAP_TYPE_PCF_SOFT )
			vec2 texelSize = vec2( 1.0 ) / shadowMapSize;
			float dx = texelSize.x;
			float dy = texelSize.y;
			vec2 uv = shadowCoord.xy;
			vec2 f = fract( uv * shadowMapSize + 0.5 );
			uv -= f * texelSize;
			shadow = (
				texture2DCompare( shadowMap, uv, shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + vec2( dx, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + vec2( 0.0, dy ), shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + texelSize, shadowCoord.z ) +
				mix( texture2DCompare( shadowMap, uv + vec2( -dx, 0.0 ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, 0.0 ), shadowCoord.z ),
					 f.x ) +
				mix( texture2DCompare( shadowMap, uv + vec2( -dx, dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, dy ), shadowCoord.z ),
					 f.x ) +
				mix( texture2DCompare( shadowMap, uv + vec2( 0.0, -dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 0.0, 2.0 * dy ), shadowCoord.z ),
					 f.y ) +
				mix( texture2DCompare( shadowMap, uv + vec2( dx, -dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( dx, 2.0 * dy ), shadowCoord.z ),
					 f.y ) +
				mix( mix( texture2DCompare( shadowMap, uv + vec2( -dx, -dy ), shadowCoord.z ),
						  texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, -dy ), shadowCoord.z ),
						  f.x ),
					 mix( texture2DCompare( shadowMap, uv + vec2( -dx, 2.0 * dy ), shadowCoord.z ),
						  texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, 2.0 * dy ), shadowCoord.z ),
						  f.x ),
					 f.y )
			) * ( 1.0 / 9.0 );
		#elif defined( SHADOWMAP_TYPE_VSM )
			shadow = VSMShadow( shadowMap, shadowCoord.xy, shadowCoord.z );
		#else
			shadow = texture2DCompare( shadowMap, shadowCoord.xy, shadowCoord.z );
		#endif
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
	vec2 cubeToUV( vec3 v, float texelSizeY ) {
		vec3 absV = abs( v );
		float scaleToCube = 1.0 / max( absV.x, max( absV.y, absV.z ) );
		absV *= scaleToCube;
		v *= scaleToCube * ( 1.0 - 2.0 * texelSizeY );
		vec2 planar = v.xy;
		float almostATexel = 1.5 * texelSizeY;
		float almostOne = 1.0 - almostATexel;
		if ( absV.z >= almostOne ) {
			if ( v.z > 0.0 )
				planar.x = 4.0 - v.x;
		} else if ( absV.x >= almostOne ) {
			float signX = sign( v.x );
			planar.x = v.z * signX + 2.0 * signX;
		} else if ( absV.y >= almostOne ) {
			float signY = sign( v.y );
			planar.x = v.x + 2.0 * signY + 2.0;
			planar.y = v.z * signY - 2.0;
		}
		return vec2( 0.125, 0.25 ) * planar + vec2( 0.375, 0.75 );
	}
	float getPointShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord, float shadowCameraNear, float shadowCameraFar ) {
		float shadow = 1.0;
		vec3 lightToPosition = shadowCoord.xyz;
		
		float lightToPositionLength = length( lightToPosition );
		if ( lightToPositionLength - shadowCameraFar <= 0.0 && lightToPositionLength - shadowCameraNear >= 0.0 ) {
			float dp = ( lightToPositionLength - shadowCameraNear ) / ( shadowCameraFar - shadowCameraNear );			dp += shadowBias;
			vec3 bd3D = normalize( lightToPosition );
			vec2 texelSize = vec2( 1.0 ) / ( shadowMapSize * vec2( 4.0, 2.0 ) );
			#if defined( SHADOWMAP_TYPE_PCF ) || defined( SHADOWMAP_TYPE_PCF_SOFT ) || defined( SHADOWMAP_TYPE_VSM )
				vec2 offset = vec2( - 1, 1 ) * shadowRadius * texelSize.y;
				shadow = (
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xyy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yyy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xyx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yyx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xxy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yxy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xxx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yxx, texelSize.y ), dp )
				) * ( 1.0 / 9.0 );
			#else
				shadow = texture2DCompare( shadowMap, cubeToUV( bd3D, texelSize.y ), dp );
			#endif
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
#endif`,Ng=`#if NUM_SPOT_LIGHT_COORDS > 0
	uniform mat4 spotLightMatrix[ NUM_SPOT_LIGHT_COORDS ];
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
		uniform mat4 directionalShadowMatrix[ NUM_DIR_LIGHT_SHADOWS ];
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		uniform mat4 pointShadowMatrix[ NUM_POINT_LIGHT_SHADOWS ];
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
#endif`,Og=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
	vec3 shadowWorldNormal = inverseTransformDirection( transformedNormal, viewMatrix );
	vec4 shadowWorldPosition;
#endif
#if defined( USE_SHADOWMAP )
	#if NUM_DIR_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * directionalLightShadows[ i ].shadowNormalBias, 0 );
			vDirectionalShadowCoord[ i ] = directionalShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * pointLightShadows[ i ].shadowNormalBias, 0 );
			vPointShadowCoord[ i ] = pointShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
#endif
#if NUM_SPOT_LIGHT_COORDS > 0
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_COORDS; i ++ ) {
		shadowWorldPosition = worldPosition;
		#if ( defined( USE_SHADOWMAP ) && UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
			shadowWorldPosition.xyz += shadowWorldNormal * spotLightShadows[ i ].shadowNormalBias;
		#endif
		vSpotLightCoord[ i ] = spotLightMatrix[ i ] * shadowWorldPosition;
	}
	#pragma unroll_loop_end
#endif`,Fg=`float getShadowMask() {
	float shadow = 1.0;
	#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
		directionalLight = directionalLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( directionalShadowMap[ i ], directionalLight.shadowMapSize, directionalLight.shadowIntensity, directionalLight.shadowBias, directionalLight.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_SHADOWS; i ++ ) {
		spotLight = spotLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( spotShadowMap[ i ], spotLight.shadowMapSize, spotLight.shadowIntensity, spotLight.shadowBias, spotLight.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
	PointLightShadow pointLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
		pointLight = pointLightShadows[ i ];
		shadow *= receiveShadow ? getPointShadow( pointShadowMap[ i ], pointLight.shadowMapSize, pointLight.shadowIntensity, pointLight.shadowBias, pointLight.shadowRadius, vPointShadowCoord[ i ], pointLight.shadowCameraNear, pointLight.shadowCameraFar ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#endif
	return shadow;
}`,Bg=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,zg=`#ifdef USE_SKINNING
	uniform mat4 bindMatrix;
	uniform mat4 bindMatrixInverse;
	uniform highp sampler2D boneTexture;
	mat4 getBoneMatrix( const in float i ) {
		int size = textureSize( boneTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( boneTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( boneTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( boneTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( boneTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
#endif`,kg=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,Hg=`#ifdef USE_SKINNING
	mat4 skinMatrix = mat4( 0.0 );
	skinMatrix += skinWeight.x * boneMatX;
	skinMatrix += skinWeight.y * boneMatY;
	skinMatrix += skinWeight.z * boneMatZ;
	skinMatrix += skinWeight.w * boneMatW;
	skinMatrix = bindMatrixInverse * skinMatrix * bindMatrix;
	objectNormal = vec4( skinMatrix * vec4( objectNormal, 0.0 ) ).xyz;
	#ifdef USE_TANGENT
		objectTangent = vec4( skinMatrix * vec4( objectTangent, 0.0 ) ).xyz;
	#endif
#endif`,Vg=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,Gg=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,Wg=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,Xg=`#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
uniform float toneMappingExposure;
vec3 LinearToneMapping( vec3 color ) {
	return saturate( toneMappingExposure * color );
}
vec3 ReinhardToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	return saturate( color / ( vec3( 1.0 ) + color ) );
}
vec3 CineonToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	color = max( vec3( 0.0 ), color - 0.004 );
	return pow( ( color * ( 6.2 * color + 0.5 ) ) / ( color * ( 6.2 * color + 1.7 ) + 0.06 ), vec3( 2.2 ) );
}
vec3 RRTAndODTFit( vec3 v ) {
	vec3 a = v * ( v + 0.0245786 ) - 0.000090537;
	vec3 b = v * ( 0.983729 * v + 0.4329510 ) + 0.238081;
	return a / b;
}
vec3 ACESFilmicToneMapping( vec3 color ) {
	const mat3 ACESInputMat = mat3(
		vec3( 0.59719, 0.07600, 0.02840 ),		vec3( 0.35458, 0.90834, 0.13383 ),
		vec3( 0.04823, 0.01566, 0.83777 )
	);
	const mat3 ACESOutputMat = mat3(
		vec3(  1.60475, -0.10208, -0.00327 ),		vec3( -0.53108,  1.10813, -0.07276 ),
		vec3( -0.07367, -0.00605,  1.07602 )
	);
	color *= toneMappingExposure / 0.6;
	color = ACESInputMat * color;
	color = RRTAndODTFit( color );
	color = ACESOutputMat * color;
	return saturate( color );
}
const mat3 LINEAR_REC2020_TO_LINEAR_SRGB = mat3(
	vec3( 1.6605, - 0.1246, - 0.0182 ),
	vec3( - 0.5876, 1.1329, - 0.1006 ),
	vec3( - 0.0728, - 0.0083, 1.1187 )
);
const mat3 LINEAR_SRGB_TO_LINEAR_REC2020 = mat3(
	vec3( 0.6274, 0.0691, 0.0164 ),
	vec3( 0.3293, 0.9195, 0.0880 ),
	vec3( 0.0433, 0.0113, 0.8956 )
);
vec3 agxDefaultContrastApprox( vec3 x ) {
	vec3 x2 = x * x;
	vec3 x4 = x2 * x2;
	return + 15.5 * x4 * x2
		- 40.14 * x4 * x
		+ 31.96 * x4
		- 6.868 * x2 * x
		+ 0.4298 * x2
		+ 0.1191 * x
		- 0.00232;
}
vec3 AgXToneMapping( vec3 color ) {
	const mat3 AgXInsetMatrix = mat3(
		vec3( 0.856627153315983, 0.137318972929847, 0.11189821299995 ),
		vec3( 0.0951212405381588, 0.761241990602591, 0.0767994186031903 ),
		vec3( 0.0482516061458583, 0.101439036467562, 0.811302368396859 )
	);
	const mat3 AgXOutsetMatrix = mat3(
		vec3( 1.1271005818144368, - 0.1413297634984383, - 0.14132976349843826 ),
		vec3( - 0.11060664309660323, 1.157823702216272, - 0.11060664309660294 ),
		vec3( - 0.016493938717834573, - 0.016493938717834257, 1.2519364065950405 )
	);
	const float AgxMinEv = - 12.47393;	const float AgxMaxEv = 4.026069;
	color *= toneMappingExposure;
	color = LINEAR_SRGB_TO_LINEAR_REC2020 * color;
	color = AgXInsetMatrix * color;
	color = max( color, 1e-10 );	color = log2( color );
	color = ( color - AgxMinEv ) / ( AgxMaxEv - AgxMinEv );
	color = clamp( color, 0.0, 1.0 );
	color = agxDefaultContrastApprox( color );
	color = AgXOutsetMatrix * color;
	color = pow( max( vec3( 0.0 ), color ), vec3( 2.2 ) );
	color = LINEAR_REC2020_TO_LINEAR_SRGB * color;
	color = clamp( color, 0.0, 1.0 );
	return color;
}
vec3 NeutralToneMapping( vec3 color ) {
	const float StartCompression = 0.8 - 0.04;
	const float Desaturation = 0.15;
	color *= toneMappingExposure;
	float x = min( color.r, min( color.g, color.b ) );
	float offset = x < 0.08 ? x - 6.25 * x * x : 0.04;
	color -= offset;
	float peak = max( color.r, max( color.g, color.b ) );
	if ( peak < StartCompression ) return color;
	float d = 1. - StartCompression;
	float newPeak = 1. - d * d / ( peak + d - StartCompression );
	color *= newPeak / peak;
	float g = 1. - 1. / ( Desaturation * ( peak - newPeak ) + 1. );
	return mix( color, vec3( newPeak ), g );
}
vec3 CustomToneMapping( vec3 color ) { return color; }`,qg=`#ifdef USE_TRANSMISSION
	material.transmission = transmission;
	material.transmissionAlpha = 1.0;
	material.thickness = thickness;
	material.attenuationDistance = attenuationDistance;
	material.attenuationColor = attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		material.transmission *= texture2D( transmissionMap, vTransmissionMapUv ).r;
	#endif
	#ifdef USE_THICKNESSMAP
		material.thickness *= texture2D( thicknessMap, vThicknessMapUv ).g;
	#endif
	vec3 pos = vWorldPosition;
	vec3 v = normalize( cameraPosition - pos );
	vec3 n = inverseTransformDirection( normal, viewMatrix );
	vec4 transmitted = getIBLVolumeRefraction(
		n, v, material.roughness, material.diffuseColor, material.specularColor, material.specularF90,
		pos, modelMatrix, viewMatrix, projectionMatrix, material.dispersion, material.ior, material.thickness,
		material.attenuationColor, material.attenuationDistance );
	material.transmissionAlpha = mix( material.transmissionAlpha, transmitted.a, material.transmission );
	totalDiffuse = mix( totalDiffuse, transmitted.rgb, material.transmission );
#endif`,Yg=`#ifdef USE_TRANSMISSION
	uniform float transmission;
	uniform float thickness;
	uniform float attenuationDistance;
	uniform vec3 attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		uniform sampler2D transmissionMap;
	#endif
	#ifdef USE_THICKNESSMAP
		uniform sampler2D thicknessMap;
	#endif
	uniform vec2 transmissionSamplerSize;
	uniform sampler2D transmissionSamplerMap;
	uniform mat4 modelMatrix;
	uniform mat4 projectionMatrix;
	varying vec3 vWorldPosition;
	float w0( float a ) {
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - a + 3.0 ) - 3.0 ) + 1.0 );
	}
	float w1( float a ) {
		return ( 1.0 / 6.0 ) * ( a *  a * ( 3.0 * a - 6.0 ) + 4.0 );
	}
	float w2( float a ){
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - 3.0 * a + 3.0 ) + 3.0 ) + 1.0 );
	}
	float w3( float a ) {
		return ( 1.0 / 6.0 ) * ( a * a * a );
	}
	float g0( float a ) {
		return w0( a ) + w1( a );
	}
	float g1( float a ) {
		return w2( a ) + w3( a );
	}
	float h0( float a ) {
		return - 1.0 + w1( a ) / ( w0( a ) + w1( a ) );
	}
	float h1( float a ) {
		return 1.0 + w3( a ) / ( w2( a ) + w3( a ) );
	}
	vec4 bicubic( sampler2D tex, vec2 uv, vec4 texelSize, float lod ) {
		uv = uv * texelSize.zw + 0.5;
		vec2 iuv = floor( uv );
		vec2 fuv = fract( uv );
		float g0x = g0( fuv.x );
		float g1x = g1( fuv.x );
		float h0x = h0( fuv.x );
		float h1x = h1( fuv.x );
		float h0y = h0( fuv.y );
		float h1y = h1( fuv.y );
		vec2 p0 = ( vec2( iuv.x + h0x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p1 = ( vec2( iuv.x + h1x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p2 = ( vec2( iuv.x + h0x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		vec2 p3 = ( vec2( iuv.x + h1x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		return g0( fuv.y ) * ( g0x * textureLod( tex, p0, lod ) + g1x * textureLod( tex, p1, lod ) ) +
			g1( fuv.y ) * ( g0x * textureLod( tex, p2, lod ) + g1x * textureLod( tex, p3, lod ) );
	}
	vec4 textureBicubic( sampler2D sampler, vec2 uv, float lod ) {
		vec2 fLodSize = vec2( textureSize( sampler, int( lod ) ) );
		vec2 cLodSize = vec2( textureSize( sampler, int( lod + 1.0 ) ) );
		vec2 fLodSizeInv = 1.0 / fLodSize;
		vec2 cLodSizeInv = 1.0 / cLodSize;
		vec4 fSample = bicubic( sampler, uv, vec4( fLodSizeInv, fLodSize ), floor( lod ) );
		vec4 cSample = bicubic( sampler, uv, vec4( cLodSizeInv, cLodSize ), ceil( lod ) );
		return mix( fSample, cSample, fract( lod ) );
	}
	vec3 getVolumeTransmissionRay( const in vec3 n, const in vec3 v, const in float thickness, const in float ior, const in mat4 modelMatrix ) {
		vec3 refractionVector = refract( - v, normalize( n ), 1.0 / ior );
		vec3 modelScale;
		modelScale.x = length( vec3( modelMatrix[ 0 ].xyz ) );
		modelScale.y = length( vec3( modelMatrix[ 1 ].xyz ) );
		modelScale.z = length( vec3( modelMatrix[ 2 ].xyz ) );
		return normalize( refractionVector ) * thickness * modelScale;
	}
	float applyIorToRoughness( const in float roughness, const in float ior ) {
		return roughness * clamp( ior * 2.0 - 2.0, 0.0, 1.0 );
	}
	vec4 getTransmissionSample( const in vec2 fragCoord, const in float roughness, const in float ior ) {
		float lod = log2( transmissionSamplerSize.x ) * applyIorToRoughness( roughness, ior );
		return textureBicubic( transmissionSamplerMap, fragCoord.xy, lod );
	}
	vec3 volumeAttenuation( const in float transmissionDistance, const in vec3 attenuationColor, const in float attenuationDistance ) {
		if ( isinf( attenuationDistance ) ) {
			return vec3( 1.0 );
		} else {
			vec3 attenuationCoefficient = -log( attenuationColor ) / attenuationDistance;
			vec3 transmittance = exp( - attenuationCoefficient * transmissionDistance );			return transmittance;
		}
	}
	vec4 getIBLVolumeRefraction( const in vec3 n, const in vec3 v, const in float roughness, const in vec3 diffuseColor,
		const in vec3 specularColor, const in float specularF90, const in vec3 position, const in mat4 modelMatrix,
		const in mat4 viewMatrix, const in mat4 projMatrix, const in float dispersion, const in float ior, const in float thickness,
		const in vec3 attenuationColor, const in float attenuationDistance ) {
		vec4 transmittedLight;
		vec3 transmittance;
		#ifdef USE_DISPERSION
			float halfSpread = ( ior - 1.0 ) * 0.025 * dispersion;
			vec3 iors = vec3( ior - halfSpread, ior, ior + halfSpread );
			for ( int i = 0; i < 3; i ++ ) {
				vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, iors[ i ], modelMatrix );
				vec3 refractedRayExit = position + transmissionRay;
		
				vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
				vec2 refractionCoords = ndcPos.xy / ndcPos.w;
				refractionCoords += 1.0;
				refractionCoords /= 2.0;
		
				vec4 transmissionSample = getTransmissionSample( refractionCoords, roughness, iors[ i ] );
				transmittedLight[ i ] = transmissionSample[ i ];
				transmittedLight.a += transmissionSample.a;
				transmittance[ i ] = diffuseColor[ i ] * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance )[ i ];
			}
			transmittedLight.a /= 3.0;
		
		#else
		
			vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, ior, modelMatrix );
			vec3 refractedRayExit = position + transmissionRay;
			vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
			vec2 refractionCoords = ndcPos.xy / ndcPos.w;
			refractionCoords += 1.0;
			refractionCoords /= 2.0;
			transmittedLight = getTransmissionSample( refractionCoords, roughness, ior );
			transmittance = diffuseColor * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance );
		
		#endif
		vec3 attenuatedColor = transmittance * transmittedLight.rgb;
		vec3 F = EnvironmentBRDF( n, v, specularColor, specularF90, roughness );
		float transmittanceFactor = ( transmittance.r + transmittance.g + transmittance.b ) / 3.0;
		return vec4( ( 1.0 - F ) * attenuatedColor, 1.0 - ( 1.0 - transmittedLight.a ) * transmittanceFactor );
	}
#endif`,Zg=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_SPECULARMAP
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,jg=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	uniform mat3 mapTransform;
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	uniform mat3 alphaMapTransform;
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	uniform mat3 lightMapTransform;
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	uniform mat3 aoMapTransform;
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	uniform mat3 bumpMapTransform;
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	uniform mat3 normalMapTransform;
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_DISPLACEMENTMAP
	uniform mat3 displacementMapTransform;
	varying vec2 vDisplacementMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	uniform mat3 emissiveMapTransform;
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	uniform mat3 metalnessMapTransform;
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	uniform mat3 roughnessMapTransform;
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	uniform mat3 anisotropyMapTransform;
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	uniform mat3 clearcoatMapTransform;
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform mat3 clearcoatNormalMapTransform;
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform mat3 clearcoatRoughnessMapTransform;
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	uniform mat3 sheenColorMapTransform;
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	uniform mat3 sheenRoughnessMapTransform;
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	uniform mat3 iridescenceMapTransform;
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform mat3 iridescenceThicknessMapTransform;
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SPECULARMAP
	uniform mat3 specularMapTransform;
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	uniform mat3 specularColorMapTransform;
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	uniform mat3 specularIntensityMapTransform;
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,Kg=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	vUv = vec3( uv, 1 ).xy;
#endif
#ifdef USE_MAP
	vMapUv = ( mapTransform * vec3( MAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ALPHAMAP
	vAlphaMapUv = ( alphaMapTransform * vec3( ALPHAMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_LIGHTMAP
	vLightMapUv = ( lightMapTransform * vec3( LIGHTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_AOMAP
	vAoMapUv = ( aoMapTransform * vec3( AOMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_BUMPMAP
	vBumpMapUv = ( bumpMapTransform * vec3( BUMPMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_NORMALMAP
	vNormalMapUv = ( normalMapTransform * vec3( NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_DISPLACEMENTMAP
	vDisplacementMapUv = ( displacementMapTransform * vec3( DISPLACEMENTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_EMISSIVEMAP
	vEmissiveMapUv = ( emissiveMapTransform * vec3( EMISSIVEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_METALNESSMAP
	vMetalnessMapUv = ( metalnessMapTransform * vec3( METALNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ROUGHNESSMAP
	vRoughnessMapUv = ( roughnessMapTransform * vec3( ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ANISOTROPYMAP
	vAnisotropyMapUv = ( anisotropyMapTransform * vec3( ANISOTROPYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOATMAP
	vClearcoatMapUv = ( clearcoatMapTransform * vec3( CLEARCOATMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	vClearcoatNormalMapUv = ( clearcoatNormalMapTransform * vec3( CLEARCOAT_NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	vClearcoatRoughnessMapUv = ( clearcoatRoughnessMapTransform * vec3( CLEARCOAT_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCEMAP
	vIridescenceMapUv = ( iridescenceMapTransform * vec3( IRIDESCENCEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	vIridescenceThicknessMapUv = ( iridescenceThicknessMapTransform * vec3( IRIDESCENCE_THICKNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_COLORMAP
	vSheenColorMapUv = ( sheenColorMapTransform * vec3( SHEEN_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	vSheenRoughnessMapUv = ( sheenRoughnessMapTransform * vec3( SHEEN_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULARMAP
	vSpecularMapUv = ( specularMapTransform * vec3( SPECULARMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_COLORMAP
	vSpecularColorMapUv = ( specularColorMapTransform * vec3( SPECULAR_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	vSpecularIntensityMapUv = ( specularIntensityMapTransform * vec3( SPECULAR_INTENSITYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_TRANSMISSIONMAP
	vTransmissionMapUv = ( transmissionMapTransform * vec3( TRANSMISSIONMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_THICKNESSMAP
	vThicknessMapUv = ( thicknessMapTransform * vec3( THICKNESSMAP_UV, 1 ) ).xy;
#endif`,Jg=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,Qg=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,$g=`uniform sampler2D t2D;
uniform float backgroundIntensity;
varying vec2 vUv;
void main() {
	vec4 texColor = texture2D( t2D, vUv );
	#ifdef DECODE_VIDEO_TEXTURE
		texColor = vec4( mix( pow( texColor.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), texColor.rgb * 0.0773993808, vec3( lessThanEqual( texColor.rgb, vec3( 0.04045 ) ) ) ), texColor.w );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,t0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,e0=`#ifdef ENVMAP_TYPE_CUBE
	uniform samplerCube envMap;
#elif defined( ENVMAP_TYPE_CUBE_UV )
	uniform sampler2D envMap;
#endif
uniform float flipEnvMap;
uniform float backgroundBlurriness;
uniform float backgroundIntensity;
uniform mat3 backgroundRotation;
varying vec3 vWorldDirection;
#include <cube_uv_reflection_fragment>
void main() {
	#ifdef ENVMAP_TYPE_CUBE
		vec4 texColor = textureCube( envMap, backgroundRotation * vec3( flipEnvMap * vWorldDirection.x, vWorldDirection.yz ) );
	#elif defined( ENVMAP_TYPE_CUBE_UV )
		vec4 texColor = textureCubeUV( envMap, backgroundRotation * vWorldDirection, backgroundBlurriness );
	#else
		vec4 texColor = vec4( 0.0, 0.0, 0.0, 1.0 );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,n0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,i0=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,s0=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
varying vec2 vHighPrecisionZW;
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vHighPrecisionZW = gl_Position.zw;
}`,r0=`#if DEPTH_PACKING == 3200
	uniform float opacity;
#endif
#include <common>
#include <packing>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
varying vec2 vHighPrecisionZW;
void main() {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#if DEPTH_PACKING == 3200
		diffuseColor.a = opacity;
	#endif
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <logdepthbuf_fragment>
	float fragCoordZ = 0.5 * vHighPrecisionZW[0] / vHighPrecisionZW[1] + 0.5;
	#if DEPTH_PACKING == 3200
		gl_FragColor = vec4( vec3( 1.0 - fragCoordZ ), opacity );
	#elif DEPTH_PACKING == 3201
		gl_FragColor = packDepthToRGBA( fragCoordZ );
	#elif DEPTH_PACKING == 3202
		gl_FragColor = vec4( packDepthToRGB( fragCoordZ ), 1.0 );
	#elif DEPTH_PACKING == 3203
		gl_FragColor = vec4( packDepthToRG( fragCoordZ ), 0.0, 1.0 );
	#endif
}`,o0=`#define DISTANCE
varying vec3 vWorldPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <worldpos_vertex>
	#include <clipping_planes_vertex>
	vWorldPosition = worldPosition.xyz;
}`,a0=`#define DISTANCE
uniform vec3 referencePosition;
uniform float nearDistance;
uniform float farDistance;
varying vec3 vWorldPosition;
#include <common>
#include <packing>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <clipping_planes_pars_fragment>
void main () {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	float dist = length( vWorldPosition - referencePosition );
	dist = ( dist - nearDistance ) / ( farDistance - nearDistance );
	dist = saturate( dist );
	gl_FragColor = packDepthToRGBA( dist );
}`,c0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,l0=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,h0=`uniform float scale;
attribute float lineDistance;
varying float vLineDistance;
#include <common>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	vLineDistance = scale * lineDistance;
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,u0=`uniform vec3 diffuse;
uniform float opacity;
uniform float dashSize;
uniform float totalSize;
varying float vLineDistance;
#include <common>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	if ( mod( vLineDistance, totalSize ) > dashSize ) {
		discard;
	}
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,d0=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#if defined ( USE_ENVMAP ) || defined ( USE_SKINNING )
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinbase_vertex>
		#include <skinnormal_vertex>
		#include <defaultnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <fog_vertex>
}`,f0=`uniform vec3 diffuse;
uniform float opacity;
#ifndef FLAT_SHADED
	varying vec3 vNormal;
#endif
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		reflectedLight.indirectDiffuse += lightMapTexel.rgb * lightMapIntensity * RECIPROCAL_PI;
	#else
		reflectedLight.indirectDiffuse += vec3( 1.0 );
	#endif
	#include <aomap_fragment>
	reflectedLight.indirectDiffuse *= diffuseColor.rgb;
	vec3 outgoingLight = reflectedLight.indirectDiffuse;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,p0=`#define LAMBERT
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,m0=`#define LAMBERT
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_lambert_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_lambert_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,g0=`#define MATCAP
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
	vViewPosition = - mvPosition.xyz;
}`,x0=`#define MATCAP
uniform vec3 diffuse;
uniform float opacity;
uniform sampler2D matcap;
varying vec3 vViewPosition;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	vec3 viewDir = normalize( vViewPosition );
	vec3 x = normalize( vec3( viewDir.z, 0.0, - viewDir.x ) );
	vec3 y = cross( viewDir, x );
	vec2 uv = vec2( dot( x, normal ), dot( y, normal ) ) * 0.495 + 0.5;
	#ifdef USE_MATCAP
		vec4 matcapColor = texture2D( matcap, uv );
	#else
		vec4 matcapColor = vec4( vec3( mix( 0.2, 0.8, uv.y ) ), 1.0 );
	#endif
	vec3 outgoingLight = diffuseColor.rgb * matcapColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,v0=`#define NORMAL
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	vViewPosition = - mvPosition.xyz;
#endif
}`,_0=`#define NORMAL
uniform float opacity;
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <packing>
#include <uv_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( 0.0, 0.0, 0.0, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	gl_FragColor = vec4( packNormalToRGB( normal ), diffuseColor.a );
	#ifdef OPAQUE
		gl_FragColor.a = 1.0;
	#endif
}`,y0=`#define PHONG
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,M0=`#define PHONG
uniform vec3 diffuse;
uniform vec3 emissive;
uniform vec3 specular;
uniform float shininess;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_phong_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_phong_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + reflectedLight.directSpecular + reflectedLight.indirectSpecular + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,b0=`#define STANDARD
varying vec3 vViewPosition;
#ifdef USE_TRANSMISSION
	varying vec3 vWorldPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
#ifdef USE_TRANSMISSION
	vWorldPosition = worldPosition.xyz;
#endif
}`,S0=`#define STANDARD
#ifdef PHYSICAL
	#define IOR
	#define USE_SPECULAR
#endif
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;
#ifdef IOR
	uniform float ior;
#endif
#ifdef USE_SPECULAR
	uniform float specularIntensity;
	uniform vec3 specularColor;
	#ifdef USE_SPECULAR_COLORMAP
		uniform sampler2D specularColorMap;
	#endif
	#ifdef USE_SPECULAR_INTENSITYMAP
		uniform sampler2D specularIntensityMap;
	#endif
#endif
#ifdef USE_CLEARCOAT
	uniform float clearcoat;
	uniform float clearcoatRoughness;
#endif
#ifdef USE_DISPERSION
	uniform float dispersion;
#endif
#ifdef USE_IRIDESCENCE
	uniform float iridescence;
	uniform float iridescenceIOR;
	uniform float iridescenceThicknessMinimum;
	uniform float iridescenceThicknessMaximum;
#endif
#ifdef USE_SHEEN
	uniform vec3 sheenColor;
	uniform float sheenRoughness;
	#ifdef USE_SHEEN_COLORMAP
		uniform sampler2D sheenColorMap;
	#endif
	#ifdef USE_SHEEN_ROUGHNESSMAP
		uniform sampler2D sheenRoughnessMap;
	#endif
#endif
#ifdef USE_ANISOTROPY
	uniform vec2 anisotropyVector;
	#ifdef USE_ANISOTROPYMAP
		uniform sampler2D anisotropyMap;
	#endif
#endif
varying vec3 vViewPosition;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <iridescence_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_physical_pars_fragment>
#include <transmission_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <iridescence_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <roughnessmap_fragment>
	#include <metalnessmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <clearcoat_normal_fragment_begin>
	#include <clearcoat_normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_physical_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 totalDiffuse = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse;
	vec3 totalSpecular = reflectedLight.directSpecular + reflectedLight.indirectSpecular;
	#include <transmission_fragment>
	vec3 outgoingLight = totalDiffuse + totalSpecular + totalEmissiveRadiance;
	#ifdef USE_SHEEN
		float sheenEnergyComp = 1.0 - 0.157 * max3( material.sheenColor );
		outgoingLight = outgoingLight * sheenEnergyComp + sheenSpecularDirect + sheenSpecularIndirect;
	#endif
	#ifdef USE_CLEARCOAT
		float dotNVcc = saturate( dot( geometryClearcoatNormal, geometryViewDir ) );
		vec3 Fcc = F_Schlick( material.clearcoatF0, material.clearcoatF90, dotNVcc );
		outgoingLight = outgoingLight * ( 1.0 - material.clearcoat * Fcc ) + ( clearcoatSpecularDirect + clearcoatSpecularIndirect ) * material.clearcoat;
	#endif
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,w0=`#define TOON
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,A0=`#define TOON
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <gradientmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_toon_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_toon_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,E0=`uniform float size;
uniform float scale;
#include <common>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
#ifdef USE_POINTS_UV
	varying vec2 vUv;
	uniform mat3 uvTransform;
#endif
void main() {
	#ifdef USE_POINTS_UV
		vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	#endif
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	gl_PointSize = size;
	#ifdef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) gl_PointSize *= ( scale / - mvPosition.z );
	#endif
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <fog_vertex>
}`,T0=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <color_pars_fragment>
#include <map_particle_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_particle_fragment>
	#include <color_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,C0=`#include <common>
#include <batching_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <shadowmap_pars_vertex>
void main() {
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,R0=`uniform vec3 color;
uniform float opacity;
#include <common>
#include <packing>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <logdepthbuf_pars_fragment>
#include <shadowmap_pars_fragment>
#include <shadowmask_pars_fragment>
void main() {
	#include <logdepthbuf_fragment>
	gl_FragColor = vec4( color, opacity * ( 1.0 - getShadowMask() ) );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
}`,P0=`uniform float rotation;
uniform vec2 center;
#include <common>
#include <uv_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	vec4 mvPosition = modelViewMatrix[ 3 ];
	vec2 scale = vec2( length( modelMatrix[ 0 ].xyz ), length( modelMatrix[ 1 ].xyz ) );
	#ifndef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) scale *= - mvPosition.z;
	#endif
	vec2 alignedPosition = ( position.xy - ( center - vec2( 0.5 ) ) ) * scale;
	vec2 rotatedPosition;
	rotatedPosition.x = cos( rotation ) * alignedPosition.x - sin( rotation ) * alignedPosition.y;
	rotatedPosition.y = sin( rotation ) * alignedPosition.x + cos( rotation ) * alignedPosition.y;
	mvPosition.xy += rotatedPosition;
	gl_Position = projectionMatrix * mvPosition;
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,I0=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
}`,Bt={alphahash_fragment:$p,alphahash_pars_fragment:tm,alphamap_fragment:em,alphamap_pars_fragment:nm,alphatest_fragment:im,alphatest_pars_fragment:sm,aomap_fragment:rm,aomap_pars_fragment:om,batching_pars_vertex:am,batching_vertex:cm,begin_vertex:lm,beginnormal_vertex:hm,bsdfs:um,iridescence_fragment:dm,bumpmap_pars_fragment:fm,clipping_planes_fragment:pm,clipping_planes_pars_fragment:mm,clipping_planes_pars_vertex:gm,clipping_planes_vertex:xm,color_fragment:vm,color_pars_fragment:_m,color_pars_vertex:ym,color_vertex:Mm,common:bm,cube_uv_reflection_fragment:Sm,defaultnormal_vertex:wm,displacementmap_pars_vertex:Am,displacementmap_vertex:Em,emissivemap_fragment:Tm,emissivemap_pars_fragment:Cm,colorspace_fragment:Rm,colorspace_pars_fragment:Pm,envmap_fragment:Im,envmap_common_pars_fragment:Lm,envmap_pars_fragment:Dm,envmap_pars_vertex:Um,envmap_physical_pars_fragment:Xm,envmap_vertex:Nm,fog_vertex:Om,fog_pars_vertex:Fm,fog_fragment:Bm,fog_pars_fragment:zm,gradientmap_pars_fragment:km,lightmap_pars_fragment:Hm,lights_lambert_fragment:Vm,lights_lambert_pars_fragment:Gm,lights_pars_begin:Wm,lights_toon_fragment:qm,lights_toon_pars_fragment:Ym,lights_phong_fragment:Zm,lights_phong_pars_fragment:jm,lights_physical_fragment:Km,lights_physical_pars_fragment:Jm,lights_fragment_begin:Qm,lights_fragment_maps:$m,lights_fragment_end:tg,logdepthbuf_fragment:eg,logdepthbuf_pars_fragment:ng,logdepthbuf_pars_vertex:ig,logdepthbuf_vertex:sg,map_fragment:rg,map_pars_fragment:og,map_particle_fragment:ag,map_particle_pars_fragment:cg,metalnessmap_fragment:lg,metalnessmap_pars_fragment:hg,morphinstance_vertex:ug,morphcolor_vertex:dg,morphnormal_vertex:fg,morphtarget_pars_vertex:pg,morphtarget_vertex:mg,normal_fragment_begin:gg,normal_fragment_maps:xg,normal_pars_fragment:vg,normal_pars_vertex:_g,normal_vertex:yg,normalmap_pars_fragment:Mg,clearcoat_normal_fragment_begin:bg,clearcoat_normal_fragment_maps:Sg,clearcoat_pars_fragment:wg,iridescence_pars_fragment:Ag,opaque_fragment:Eg,packing:Tg,premultiplied_alpha_fragment:Cg,project_vertex:Rg,dithering_fragment:Pg,dithering_pars_fragment:Ig,roughnessmap_fragment:Lg,roughnessmap_pars_fragment:Dg,shadowmap_pars_fragment:Ug,shadowmap_pars_vertex:Ng,shadowmap_vertex:Og,shadowmask_pars_fragment:Fg,skinbase_vertex:Bg,skinning_pars_vertex:zg,skinning_vertex:kg,skinnormal_vertex:Hg,specularmap_fragment:Vg,specularmap_pars_fragment:Gg,tonemapping_fragment:Wg,tonemapping_pars_fragment:Xg,transmission_fragment:qg,transmission_pars_fragment:Yg,uv_pars_fragment:Zg,uv_pars_vertex:jg,uv_vertex:Kg,worldpos_vertex:Jg,background_vert:Qg,background_frag:$g,backgroundCube_vert:t0,backgroundCube_frag:e0,cube_vert:n0,cube_frag:i0,depth_vert:s0,depth_frag:r0,distanceRGBA_vert:o0,distanceRGBA_frag:a0,equirect_vert:c0,equirect_frag:l0,linedashed_vert:h0,linedashed_frag:u0,meshbasic_vert:d0,meshbasic_frag:f0,meshlambert_vert:p0,meshlambert_frag:m0,meshmatcap_vert:g0,meshmatcap_frag:x0,meshnormal_vert:v0,meshnormal_frag:_0,meshphong_vert:y0,meshphong_frag:M0,meshphysical_vert:b0,meshphysical_frag:S0,meshtoon_vert:w0,meshtoon_frag:A0,points_vert:E0,points_frag:T0,shadow_vert:C0,shadow_frag:R0,sprite_vert:P0,sprite_frag:I0},dt={common:{diffuse:{value:new Ot(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new zt},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new zt}},envmap:{envMap:{value:null},envMapRotation:{value:new zt},flipEnvMap:{value:-1},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new zt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new zt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new zt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new zt},normalScale:{value:new yt(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new zt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new zt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new zt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new zt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Ot(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMap:{value:[]},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotShadowMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMap:{value:[]},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null}},points:{diffuse:{value:new Ot(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0},uvTransform:{value:new zt}},sprite:{diffuse:{value:new Ot(16777215)},opacity:{value:1},center:{value:new yt(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new zt},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0}}},Mn={basic:{uniforms:De([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.fog]),vertexShader:Bt.meshbasic_vert,fragmentShader:Bt.meshbasic_frag},lambert:{uniforms:De([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)}}]),vertexShader:Bt.meshlambert_vert,fragmentShader:Bt.meshlambert_frag},phong:{uniforms:De([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)},specular:{value:new Ot(1118481)},shininess:{value:30}}]),vertexShader:Bt.meshphong_vert,fragmentShader:Bt.meshphong_frag},standard:{uniforms:De([dt.common,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.roughnessmap,dt.metalnessmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:Bt.meshphysical_vert,fragmentShader:Bt.meshphysical_frag},toon:{uniforms:De([dt.common,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.gradientmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)}}]),vertexShader:Bt.meshtoon_vert,fragmentShader:Bt.meshtoon_frag},matcap:{uniforms:De([dt.common,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,{matcap:{value:null}}]),vertexShader:Bt.meshmatcap_vert,fragmentShader:Bt.meshmatcap_frag},points:{uniforms:De([dt.points,dt.fog]),vertexShader:Bt.points_vert,fragmentShader:Bt.points_frag},dashed:{uniforms:De([dt.common,dt.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:Bt.linedashed_vert,fragmentShader:Bt.linedashed_frag},depth:{uniforms:De([dt.common,dt.displacementmap]),vertexShader:Bt.depth_vert,fragmentShader:Bt.depth_frag},normal:{uniforms:De([dt.common,dt.bumpmap,dt.normalmap,dt.displacementmap,{opacity:{value:1}}]),vertexShader:Bt.meshnormal_vert,fragmentShader:Bt.meshnormal_frag},sprite:{uniforms:De([dt.sprite,dt.fog]),vertexShader:Bt.sprite_vert,fragmentShader:Bt.sprite_frag},background:{uniforms:{uvTransform:{value:new zt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:Bt.background_vert,fragmentShader:Bt.background_frag},backgroundCube:{uniforms:{envMap:{value:null},flipEnvMap:{value:-1},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new zt}},vertexShader:Bt.backgroundCube_vert,fragmentShader:Bt.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:Bt.cube_vert,fragmentShader:Bt.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:Bt.equirect_vert,fragmentShader:Bt.equirect_frag},distanceRGBA:{uniforms:De([dt.common,dt.displacementmap,{referencePosition:{value:new D},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:Bt.distanceRGBA_vert,fragmentShader:Bt.distanceRGBA_frag},shadow:{uniforms:De([dt.lights,dt.fog,{color:{value:new Ot(0)},opacity:{value:1}}]),vertexShader:Bt.shadow_vert,fragmentShader:Bt.shadow_frag}};Mn.physical={uniforms:De([Mn.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new zt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new zt},clearcoatNormalScale:{value:new yt(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new zt},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new zt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new zt},sheen:{value:0},sheenColor:{value:new Ot(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new zt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new zt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new zt},transmissionSamplerSize:{value:new yt},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new zt},attenuationDistance:{value:0},attenuationColor:{value:new Ot(0)},specularColor:{value:new Ot(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new zt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new zt},anisotropyVector:{value:new yt},anisotropyMap:{value:null},anisotropyMapTransform:{value:new zt}}]),vertexShader:Bt.meshphysical_vert,fragmentShader:Bt.meshphysical_frag};var no={r:0,b:0,g:0},_i=new nn,L0=new $t;function D0(n,t,e,i,s,r,o){let a=new Ot(0),c=r===!0?0:1,l,f,d=null,h=0,m=null;function x(b){let y=b.isScene===!0?b.background:null;return y&&y.isTexture&&(y=(b.backgroundBlurriness>0?e:t).get(y)),y}function _(b){let y=!1,A=x(b);A===null?p(a,c):A&&A.isColor&&(p(A,1),y=!0);let R=n.xr.getEnvironmentBlendMode();R==="additive"?i.buffers.color.setClear(0,0,0,1,o):R==="alpha-blend"&&i.buffers.color.setClear(0,0,0,0,o),(n.autoClear||y)&&(i.buffers.depth.setTest(!0),i.buffers.depth.setMask(!0),i.buffers.color.setMask(!0),n.clear(n.autoClearColor,n.autoClearDepth,n.autoClearStencil))}function u(b,y){let A=x(y);A&&(A.isCubeTexture||A.mapping===Ho)?(f===void 0&&(f=new Oe(new hr(1,1,1),new te({name:"BackgroundCubeMaterial",uniforms:Ss(Mn.backgroundCube.uniforms),vertexShader:Mn.backgroundCube.vertexShader,fragmentShader:Mn.backgroundCube.fragmentShader,side:ze,depthTest:!1,depthWrite:!1,fog:!1})),f.geometry.deleteAttribute("normal"),f.geometry.deleteAttribute("uv"),f.onBeforeRender=function(R,T,E){this.matrixWorld.copyPosition(E.matrixWorld)},Object.defineProperty(f.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),s.update(f)),_i.copy(y.backgroundRotation),_i.x*=-1,_i.y*=-1,_i.z*=-1,A.isCubeTexture&&A.isRenderTargetTexture===!1&&(_i.y*=-1,_i.z*=-1),f.material.uniforms.envMap.value=A,f.material.uniforms.flipEnvMap.value=A.isCubeTexture&&A.isRenderTargetTexture===!1?-1:1,f.material.uniforms.backgroundBlurriness.value=y.backgroundBlurriness,f.material.uniforms.backgroundIntensity.value=y.backgroundIntensity,f.material.uniforms.backgroundRotation.value.setFromMatrix4(L0.makeRotationFromEuler(_i)),f.material.toneMapped=Jt.getTransfer(A.colorSpace)!==ie,(d!==A||h!==A.version||m!==n.toneMapping)&&(f.material.needsUpdate=!0,d=A,h=A.version,m=n.toneMapping),f.layers.enableAll(),b.unshift(f,f.geometry,f.material,0,0,null)):A&&A.isTexture&&(l===void 0&&(l=new Oe(new To(2,2),new te({name:"BackgroundMaterial",uniforms:Ss(Mn.background.uniforms),vertexShader:Mn.background.vertexShader,fragmentShader:Mn.background.fragmentShader,side:ei,depthTest:!1,depthWrite:!1,fog:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),s.update(l)),l.material.uniforms.t2D.value=A,l.material.uniforms.backgroundIntensity.value=y.backgroundIntensity,l.material.toneMapped=Jt.getTransfer(A.colorSpace)!==ie,A.matrixAutoUpdate===!0&&A.updateMatrix(),l.material.uniforms.uvTransform.value.copy(A.matrix),(d!==A||h!==A.version||m!==n.toneMapping)&&(l.material.needsUpdate=!0,d=A,h=A.version,m=n.toneMapping),l.layers.enableAll(),b.unshift(l,l.geometry,l.material,0,0,null))}function p(b,y){b.getRGB(no,nd(n)),i.buffers.color.setClear(no.r,no.g,no.b,y,o)}return{getClearColor:function(){return a},setClearColor:function(b,y=1){a.set(b),c=y,p(a,c)},getClearAlpha:function(){return c},setClearAlpha:function(b){c=b,p(a,c)},render:_,addToRenderList:u}}function U0(n,t){let e=n.getParameter(n.MAX_VERTEX_ATTRIBS),i={},s=h(null),r=s,o=!1;function a(g,S,F,V,q){let et=!1,B=d(V,F,S);r!==B&&(r=B,l(r.object)),et=m(g,V,F,q),et&&x(g,V,F,q),q!==null&&t.update(q,n.ELEMENT_ARRAY_BUFFER),(et||o)&&(o=!1,A(g,S,F,V),q!==null&&n.bindBuffer(n.ELEMENT_ARRAY_BUFFER,t.get(q).buffer))}function c(){return n.createVertexArray()}function l(g){return n.bindVertexArray(g)}function f(g){return n.deleteVertexArray(g)}function d(g,S,F){let V=F.wireframe===!0,q=i[g.id];q===void 0&&(q={},i[g.id]=q);let et=q[S.id];et===void 0&&(et={},q[S.id]=et);let B=et[V];return B===void 0&&(B=h(c()),et[V]=B),B}function h(g){let S=[],F=[],V=[];for(let q=0;q<e;q++)S[q]=0,F[q]=0,V[q]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:S,enabledAttributes:F,attributeDivisors:V,object:g,attributes:{},index:null}}function m(g,S,F,V){let q=r.attributes,et=S.attributes,B=0,it=F.getAttributes();for(let Z in it)if(it[Z].location>=0){let _t=q[Z],bt=et[Z];if(bt===void 0&&(Z==="instanceMatrix"&&g.instanceMatrix&&(bt=g.instanceMatrix),Z==="instanceColor"&&g.instanceColor&&(bt=g.instanceColor)),_t===void 0||_t.attribute!==bt||bt&&_t.data!==bt.data)return!0;B++}return r.attributesNum!==B||r.index!==V}function x(g,S,F,V){let q={},et=S.attributes,B=0,it=F.getAttributes();for(let Z in it)if(it[Z].location>=0){let _t=et[Z];_t===void 0&&(Z==="instanceMatrix"&&g.instanceMatrix&&(_t=g.instanceMatrix),Z==="instanceColor"&&g.instanceColor&&(_t=g.instanceColor));let bt={};bt.attribute=_t,_t&&_t.data&&(bt.data=_t.data),q[Z]=bt,B++}r.attributes=q,r.attributesNum=B,r.index=V}function _(){let g=r.newAttributes;for(let S=0,F=g.length;S<F;S++)g[S]=0}function u(g){p(g,0)}function p(g,S){let F=r.newAttributes,V=r.enabledAttributes,q=r.attributeDivisors;F[g]=1,V[g]===0&&(n.enableVertexAttribArray(g),V[g]=1),q[g]!==S&&(n.vertexAttribDivisor(g,S),q[g]=S)}function b(){let g=r.newAttributes,S=r.enabledAttributes;for(let F=0,V=S.length;F<V;F++)S[F]!==g[F]&&(n.disableVertexAttribArray(F),S[F]=0)}function y(g,S,F,V,q,et,B){B===!0?n.vertexAttribIPointer(g,S,F,q,et):n.vertexAttribPointer(g,S,F,V,q,et)}function A(g,S,F,V){_();let q=V.attributes,et=F.getAttributes(),B=S.defaultAttributeValues;for(let it in et){let Z=et[it];if(Z.location>=0){let gt=q[it];if(gt===void 0&&(it==="instanceMatrix"&&g.instanceMatrix&&(gt=g.instanceMatrix),it==="instanceColor"&&g.instanceColor&&(gt=g.instanceColor)),gt!==void 0){let _t=gt.normalized,bt=gt.itemSize,Gt=t.get(gt);if(Gt===void 0)continue;let Wt=Gt.buffer,J=Gt.type,st=Gt.bytesPerElement,Mt=J===n.INT||J===n.UNSIGNED_INT||gt.gpuType===Ul;if(gt.isInterleavedBufferAttribute){let xt=gt.data,Nt=xt.stride,It=gt.offset;if(xt.isInstancedInterleavedBuffer){for(let kt=0;kt<Z.locationSize;kt++)p(Z.location+kt,xt.meshPerAttribute);g.isInstancedMesh!==!0&&V._maxInstanceCount===void 0&&(V._maxInstanceCount=xt.meshPerAttribute*xt.count)}else for(let kt=0;kt<Z.locationSize;kt++)u(Z.location+kt);n.bindBuffer(n.ARRAY_BUFFER,Wt);for(let kt=0;kt<Z.locationSize;kt++)y(Z.location+kt,bt/Z.locationSize,J,_t,Nt*st,(It+bt/Z.locationSize*kt)*st,Mt)}else{if(gt.isInstancedBufferAttribute){for(let xt=0;xt<Z.locationSize;xt++)p(Z.location+xt,gt.meshPerAttribute);g.isInstancedMesh!==!0&&V._maxInstanceCount===void 0&&(V._maxInstanceCount=gt.meshPerAttribute*gt.count)}else for(let xt=0;xt<Z.locationSize;xt++)u(Z.location+xt);n.bindBuffer(n.ARRAY_BUFFER,Wt);for(let xt=0;xt<Z.locationSize;xt++)y(Z.location+xt,bt/Z.locationSize,J,_t,bt*st,bt/Z.locationSize*xt*st,Mt)}}else if(B!==void 0){let _t=B[it];if(_t!==void 0)switch(_t.length){case 2:n.vertexAttrib2fv(Z.location,_t);break;case 3:n.vertexAttrib3fv(Z.location,_t);break;case 4:n.vertexAttrib4fv(Z.location,_t);break;default:n.vertexAttrib1fv(Z.location,_t)}}}}b()}function R(){C();for(let g in i){let S=i[g];for(let F in S){let V=S[F];for(let q in V)f(V[q].object),delete V[q];delete S[F]}delete i[g]}}function T(g){if(i[g.id]===void 0)return;let S=i[g.id];for(let F in S){let V=S[F];for(let q in V)f(V[q].object),delete V[q];delete S[F]}delete i[g.id]}function E(g){for(let S in i){let F=i[S];if(F[g.id]===void 0)continue;let V=F[g.id];for(let q in V)f(V[q].object),delete V[q];delete F[g.id]}}function C(){N(),o=!0,r!==s&&(r=s,l(r.object))}function N(){s.geometry=null,s.program=null,s.wireframe=!1}return{setup:a,reset:C,resetDefaultState:N,dispose:R,releaseStatesOfGeometry:T,releaseStatesOfProgram:E,initAttributes:_,enableAttribute:u,disableUnusedAttributes:b}}function N0(n,t,e){let i;function s(l){i=l}function r(l,f){n.drawArrays(i,l,f),e.update(f,i,1)}function o(l,f,d){d!==0&&(n.drawArraysInstanced(i,l,f,d),e.update(f,i,d))}function a(l,f,d){if(d===0)return;t.get("WEBGL_multi_draw").multiDrawArraysWEBGL(i,l,0,f,0,d);let m=0;for(let x=0;x<d;x++)m+=f[x];e.update(m,i,1)}function c(l,f,d,h){if(d===0)return;let m=t.get("WEBGL_multi_draw");if(m===null)for(let x=0;x<l.length;x++)o(l[x],f[x],h[x]);else{m.multiDrawArraysInstancedWEBGL(i,l,0,f,0,h,0,d);let x=0;for(let _=0;_<d;_++)x+=f[_];for(let _=0;_<h.length;_++)e.update(x,i,h[_])}}this.setMode=s,this.render=r,this.renderInstances=o,this.renderMultiDraw=a,this.renderMultiDrawInstances=c}function O0(n,t,e,i){let s;function r(){if(s!==void 0)return s;if(t.has("EXT_texture_filter_anisotropic")===!0){let E=t.get("EXT_texture_filter_anisotropic");s=n.getParameter(E.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else s=0;return s}function o(E){return!(E!==fn&&i.convert(E)!==n.getParameter(n.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(E){let C=E===Ve&&(t.has("EXT_color_buffer_half_float")||t.has("EXT_color_buffer_float"));return!(E!==Fn&&i.convert(E)!==n.getParameter(n.IMPLEMENTATION_COLOR_READ_TYPE)&&E!==bn&&!C)}function c(E){if(E==="highp"){if(n.getShaderPrecisionFormat(n.VERTEX_SHADER,n.HIGH_FLOAT).precision>0&&n.getShaderPrecisionFormat(n.FRAGMENT_SHADER,n.HIGH_FLOAT).precision>0)return"highp";E="mediump"}return E==="mediump"&&n.getShaderPrecisionFormat(n.VERTEX_SHADER,n.MEDIUM_FLOAT).precision>0&&n.getShaderPrecisionFormat(n.FRAGMENT_SHADER,n.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let l=e.precision!==void 0?e.precision:"highp",f=c(l);f!==l&&(console.warn("THREE.WebGLRenderer:",l,"not supported, using",f,"instead."),l=f);let d=e.logarithmicDepthBuffer===!0,h=e.reverseDepthBuffer===!0&&t.has("EXT_clip_control");if(h===!0){let E=t.get("EXT_clip_control");E.clipControlEXT(E.LOWER_LEFT_EXT,E.ZERO_TO_ONE_EXT)}let m=n.getParameter(n.MAX_TEXTURE_IMAGE_UNITS),x=n.getParameter(n.MAX_VERTEX_TEXTURE_IMAGE_UNITS),_=n.getParameter(n.MAX_TEXTURE_SIZE),u=n.getParameter(n.MAX_CUBE_MAP_TEXTURE_SIZE),p=n.getParameter(n.MAX_VERTEX_ATTRIBS),b=n.getParameter(n.MAX_VERTEX_UNIFORM_VECTORS),y=n.getParameter(n.MAX_VARYING_VECTORS),A=n.getParameter(n.MAX_FRAGMENT_UNIFORM_VECTORS),R=x>0,T=n.getParameter(n.MAX_SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:c,textureFormatReadable:o,textureTypeReadable:a,precision:l,logarithmicDepthBuffer:d,reverseDepthBuffer:h,maxTextures:m,maxVertexTextures:x,maxTextureSize:_,maxCubemapSize:u,maxAttributes:p,maxVertexUniforms:b,maxVaryings:y,maxFragmentUniforms:A,vertexTextures:R,maxSamples:T}}function F0(n){let t=this,e=null,i=0,s=!1,r=!1,o=new dn,a=new zt,c={value:null,needsUpdate:!1};this.uniform=c,this.numPlanes=0,this.numIntersection=0,this.init=function(d,h){let m=d.length!==0||h||i!==0||s;return s=h,i=d.length,m},this.beginShadows=function(){r=!0,f(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(d,h){e=f(d,h,0)},this.setState=function(d,h,m){let x=d.clippingPlanes,_=d.clipIntersection,u=d.clipShadows,p=n.get(d);if(!s||x===null||x.length===0||r&&!u)r?f(null):l();else{let b=r?0:i,y=b*4,A=p.clippingState||null;c.value=A,A=f(x,h,y,m);for(let R=0;R!==y;++R)A[R]=e[R];p.clippingState=A,this.numIntersection=_?this.numPlanes:0,this.numPlanes+=b}};function l(){c.value!==e&&(c.value=e,c.needsUpdate=i>0),t.numPlanes=i,t.numIntersection=0}function f(d,h,m,x){let _=d!==null?d.length:0,u=null;if(_!==0){if(u=c.value,x!==!0||u===null){let p=m+_*4,b=h.matrixWorldInverse;a.getNormalMatrix(b),(u===null||u.length<p)&&(u=new Float32Array(p));for(let y=0,A=m;y!==_;++y,A+=4)o.copy(d[y]).applyMatrix4(b,a),o.normal.toArray(u,A),u[A+3]=o.constant}c.value=u,c.needsUpdate=!0}return t.numPlanes=_,t.numIntersection=0,u}}function B0(n){let t=new WeakMap;function e(o,a){return a===Ec?o.mapping=vs:a===Tc&&(o.mapping=_s),o}function i(o){if(o&&o.isTexture){let a=o.mapping;if(a===Ec||a===Tc)if(t.has(o)){let c=t.get(o).texture;return e(c,o.mapping)}else{let c=o.image;if(c&&c.height>0){let l=new al(c.height);return l.fromEquirectangularTexture(n,o),t.set(o,l),o.addEventListener("dispose",s),e(l.texture,o.mapping)}else return null}}return o}function s(o){let a=o.target;a.removeEventListener("dispose",s);let c=t.get(a);c!==void 0&&(t.delete(a),c.dispose())}function r(){t=new WeakMap}return{get:i,dispose:r}}var ws=class extends Ao{constructor(t=-1,e=1,i=1,s=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=t,this.right=e,this.top=i,this.bottom=s,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(t,e){return super.copy(t,e),this.left=t.left,this.right=t.right,this.top=t.top,this.bottom=t.bottom,this.near=t.near,this.far=t.far,this.zoom=t.zoom,this.view=t.view===null?null:Object.assign({},t.view),this}setViewOffset(t,e,i,s,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=t,this.view.fullHeight=e,this.view.offsetX=i,this.view.offsetY=s,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let t=(this.right-this.left)/(2*this.zoom),e=(this.top-this.bottom)/(2*this.zoom),i=(this.right+this.left)/2,s=(this.top+this.bottom)/2,r=i-t,o=i+t,a=s+e,c=s-e;if(this.view!==null&&this.view.enabled){let l=(this.right-this.left)/this.view.fullWidth/this.zoom,f=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=l*this.view.offsetX,o=r+l*this.view.width,a-=f*this.view.offsetY,c=a-f*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,c,this.near,this.far,this.coordinateSystem),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(t){let e=super.toJSON(t);return e.object.zoom=this.zoom,e.object.left=this.left,e.object.right=this.right,e.object.top=this.top,e.object.bottom=this.bottom,e.object.near=this.near,e.object.far=this.far,this.view!==null&&(e.object.view=Object.assign({},this.view)),e}},ds=4,hu=[.125,.215,.35,.446,.526,.582],wi=20,hc=new ws,uu=new Ot,uc=null,dc=0,fc=0,pc=!1,Mi=(1+Math.sqrt(5))/2,ls=1/Mi,du=[new D(-Mi,ls,0),new D(Mi,ls,0),new D(-ls,0,Mi),new D(ls,0,Mi),new D(0,Mi,-ls),new D(0,Mi,ls),new D(-1,1,-1),new D(1,1,-1),new D(-1,1,1),new D(1,1,1)],Co=class{constructor(t){this._renderer=t,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._lodPlanes=[],this._sizeLods=[],this._sigmas=[],this._blurMaterial=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._compileMaterial(this._blurMaterial)}fromScene(t,e=0,i=.1,s=100){uc=this._renderer.getRenderTarget(),dc=this._renderer.getActiveCubeFace(),fc=this._renderer.getActiveMipmapLevel(),pc=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(256);let r=this._allocateTargets();return r.depthBuffer=!0,this._sceneToCubeUV(t,i,s,r),e>0&&this._blur(r,0,0,e),this._applyPMREM(r),this._cleanup(r),r}fromEquirectangular(t,e=null){return this._fromTexture(t,e)}fromCubemap(t,e=null){return this._fromTexture(t,e)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=mu(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=pu(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose()}_setSize(t){this._lodMax=Math.floor(Math.log2(t)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let t=0;t<this._lodPlanes.length;t++)this._lodPlanes[t].dispose()}_cleanup(t){this._renderer.setRenderTarget(uc,dc,fc),this._renderer.xr.enabled=pc,t.scissorTest=!1,io(t,0,0,t.width,t.height)}_fromTexture(t,e){t.mapping===vs||t.mapping===_s?this._setSize(t.image.length===0?16:t.image[0].width||t.image[0].image.width):this._setSize(t.image.width/4),uc=this._renderer.getRenderTarget(),dc=this._renderer.getActiveCubeFace(),fc=this._renderer.getActiveMipmapLevel(),pc=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let i=e||this._allocateTargets();return this._textureToCubeUV(t,i),this._applyPMREM(i),this._cleanup(i),i}_allocateTargets(){let t=3*Math.max(this._cubeSize,112),e=4*this._cubeSize,i={magFilter:je,minFilter:je,generateMipmaps:!1,type:Ve,format:fn,colorSpace:si,depthBuffer:!1},s=fu(t,e,i);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==t||this._pingPongRenderTarget.height!==e){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=fu(t,e,i);let{_lodMax:r}=this;({sizeLods:this._sizeLods,lodPlanes:this._lodPlanes,sigmas:this._sigmas}=z0(r)),this._blurMaterial=k0(r,t,e)}return s}_compileMaterial(t){let e=new Oe(this._lodPlanes[0],t);this._renderer.compile(e,hc)}_sceneToCubeUV(t,e,i,s){let a=new Ne(90,1,e,i),c=[1,-1,1,1,1,1],l=[1,1,1,-1,-1,-1],f=this._renderer,d=f.autoClear,h=f.toneMapping;f.getClearColor(uu),f.toneMapping=ti,f.autoClear=!1;let m=new bs({name:"PMREM.Background",side:ze,depthWrite:!1,depthTest:!1}),x=new Oe(new hr,m),_=!1,u=t.background;u?u.isColor&&(m.color.copy(u),t.background=null,_=!0):(m.color.copy(uu),_=!0);for(let p=0;p<6;p++){let b=p%3;b===0?(a.up.set(0,c[p],0),a.lookAt(l[p],0,0)):b===1?(a.up.set(0,0,c[p]),a.lookAt(0,l[p],0)):(a.up.set(0,c[p],0),a.lookAt(0,0,l[p]));let y=this._cubeSize;io(s,b*y,p>2?y:0,y,y),f.setRenderTarget(s),_&&f.render(x,a),f.render(t,a)}x.geometry.dispose(),x.material.dispose(),f.toneMapping=h,f.autoClear=d,t.background=u}_textureToCubeUV(t,e){let i=this._renderer,s=t.mapping===vs||t.mapping===_s;s?(this._cubemapMaterial===null&&(this._cubemapMaterial=mu()),this._cubemapMaterial.uniforms.flipEnvMap.value=t.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=pu());let r=s?this._cubemapMaterial:this._equirectMaterial,o=new Oe(this._lodPlanes[0],r),a=r.uniforms;a.envMap.value=t;let c=this._cubeSize;io(e,0,0,3*c,2*c),i.setRenderTarget(e),i.render(o,hc)}_applyPMREM(t){let e=this._renderer,i=e.autoClear;e.autoClear=!1;let s=this._lodPlanes.length;for(let r=1;r<s;r++){let o=Math.sqrt(this._sigmas[r]*this._sigmas[r]-this._sigmas[r-1]*this._sigmas[r-1]),a=du[(s-r-1)%du.length];this._blur(t,r-1,r,o,a)}e.autoClear=i}_blur(t,e,i,s,r){let o=this._pingPongRenderTarget;this._halfBlur(t,o,e,i,s,"latitudinal",r),this._halfBlur(o,t,i,i,s,"longitudinal",r)}_halfBlur(t,e,i,s,r,o,a){let c=this._renderer,l=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&console.error("blur direction must be either latitudinal or longitudinal!");let f=3,d=new Oe(this._lodPlanes[s],l),h=l.uniforms,m=this._sizeLods[i]-1,x=isFinite(r)?Math.PI/(2*m):2*Math.PI/(2*wi-1),_=r/x,u=isFinite(r)?1+Math.floor(f*_):wi;u>wi&&console.warn(`sigmaRadians, ${r}, is too large and will clip, as it requested ${u} samples when the maximum is set to ${wi}`);let p=[],b=0;for(let E=0;E<wi;++E){let C=E/_,N=Math.exp(-C*C/2);p.push(N),E===0?b+=N:E<u&&(b+=2*N)}for(let E=0;E<p.length;E++)p[E]=p[E]/b;h.envMap.value=t.texture,h.samples.value=u,h.weights.value=p,h.latitudinal.value=o==="latitudinal",a&&(h.poleAxis.value=a);let{_lodMax:y}=this;h.dTheta.value=x,h.mipInt.value=y-i;let A=this._sizeLods[s],R=3*A*(s>y-ds?s-y+ds:0),T=4*(this._cubeSize-A);io(e,R,T,3*A,2*A),c.setRenderTarget(e),c.render(d,hc)}};function z0(n){let t=[],e=[],i=[],s=n,r=n-ds+1+hu.length;for(let o=0;o<r;o++){let a=Math.pow(2,s);e.push(a);let c=1/a;o>n-ds?c=hu[o-n+ds-1]:o===0&&(c=0),i.push(c);let l=1/(a-2),f=-l,d=1+l,h=[f,f,d,f,d,d,f,f,d,d,f,d],m=6,x=6,_=3,u=2,p=1,b=new Float32Array(_*x*m),y=new Float32Array(u*x*m),A=new Float32Array(p*x*m);for(let T=0;T<m;T++){let E=T%3*2/3-1,C=T>2?0:-1,N=[E,C,0,E+2/3,C,0,E+2/3,C+1,0,E,C,0,E+2/3,C+1,0,E,C+1,0];b.set(N,_*x*T),y.set(h,u*x*T);let g=[T,T,T,T,T,T];A.set(g,p*x*T)}let R=new pn;R.setAttribute("position",new Ke(b,_)),R.setAttribute("uv",new Ke(y,u)),R.setAttribute("faceIndex",new Ke(A,p)),t.push(R),s>ds&&s--}return{lodPlanes:t,sizeLods:e,sigmas:i}}function fu(n,t,e){let i=new fe(n,t,e);return i.texture.mapping=Ho,i.texture.name="PMREM.cubeUv",i.scissorTest=!0,i}function io(n,t,e,i,s){n.viewport.set(t,e,i,s),n.scissor.set(t,e,i,s)}function k0(n,t,e){let i=new Float32Array(wi),s=new D(0,1,0);return new te({name:"SphericalGaussianBlur",defines:{n:wi,CUBEUV_TEXEL_WIDTH:1/t,CUBEUV_TEXEL_HEIGHT:1/e,CUBEUV_MAX_MIP:`${n}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:i},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:s}},vertexShader:Wl(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;
			uniform int samples;
			uniform float weights[ n ];
			uniform bool latitudinal;
			uniform float dTheta;
			uniform float mipInt;
			uniform vec3 poleAxis;

			#define ENVMAP_TYPE_CUBE_UV
			#include <cube_uv_reflection_fragment>

			vec3 getSample( float theta, vec3 axis ) {

				float cosTheta = cos( theta );
				// Rodrigues' axis-angle rotation
				vec3 sampleDirection = vOutputDirection * cosTheta
					+ cross( axis, vOutputDirection ) * sin( theta )
					+ axis * dot( axis, vOutputDirection ) * ( 1.0 - cosTheta );

				return bilinearCubeUV( envMap, sampleDirection, mipInt );

			}

			void main() {

				vec3 axis = latitudinal ? poleAxis : cross( poleAxis, vOutputDirection );

				if ( all( equal( axis, vec3( 0.0 ) ) ) ) {

					axis = vec3( vOutputDirection.z, 0.0, - vOutputDirection.x );

				}

				axis = normalize( axis );

				gl_FragColor = vec4( 0.0, 0.0, 0.0, 1.0 );
				gl_FragColor.rgb += weights[ 0 ] * getSample( 0.0, axis );

				for ( int i = 1; i < n; i++ ) {

					if ( i >= samples ) {

						break;

					}

					float theta = dTheta * float( i );
					gl_FragColor.rgb += weights[ i ] * getSample( -1.0 * theta, axis );
					gl_FragColor.rgb += weights[ i ] * getSample( theta, axis );

				}

			}
		`,blending:Sn,depthTest:!1,depthWrite:!1})}function pu(){return new te({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:Wl(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;

			#include <common>

			void main() {

				vec3 outputDirection = normalize( vOutputDirection );
				vec2 uv = equirectUv( outputDirection );

				gl_FragColor = vec4( texture2D ( envMap, uv ).rgb, 1.0 );

			}
		`,blending:Sn,depthTest:!1,depthWrite:!1})}function mu(){return new te({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:Wl(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:Sn,depthTest:!1,depthWrite:!1})}function Wl(){return`

		precision mediump float;
		precision mediump int;

		attribute float faceIndex;

		varying vec3 vOutputDirection;

		// RH coordinate system; PMREM face-indexing convention
		vec3 getDirection( vec2 uv, float face ) {

			uv = 2.0 * uv - 1.0;

			vec3 direction = vec3( uv, 1.0 );

			if ( face == 0.0 ) {

				direction = direction.zyx; // ( 1, v, u ) pos x

			} else if ( face == 1.0 ) {

				direction = direction.xzy;
				direction.xz *= -1.0; // ( -u, 1, -v ) pos y

			} else if ( face == 2.0 ) {

				direction.x *= -1.0; // ( -u, v, 1 ) pos z

			} else if ( face == 3.0 ) {

				direction = direction.zyx;
				direction.xz *= -1.0; // ( -1, v, -u ) neg x

			} else if ( face == 4.0 ) {

				direction = direction.xzy;
				direction.xy *= -1.0; // ( -u, -1, v ) neg y

			} else if ( face == 5.0 ) {

				direction.z *= -1.0; // ( u, v, -1 ) neg z

			}

			return direction;

		}

		void main() {

			vOutputDirection = getDirection( uv, faceIndex );
			gl_Position = vec4( position, 1.0 );

		}
	`}function H0(n){let t=new WeakMap,e=null;function i(a){if(a&&a.isTexture){let c=a.mapping,l=c===Ec||c===Tc,f=c===vs||c===_s;if(l||f){let d=t.get(a),h=d!==void 0?d.texture.pmremVersion:0;if(a.isRenderTargetTexture&&a.pmremVersion!==h)return e===null&&(e=new Co(n)),d=l?e.fromEquirectangular(a,d):e.fromCubemap(a,d),d.texture.pmremVersion=a.pmremVersion,t.set(a,d),d.texture;if(d!==void 0)return d.texture;{let m=a.image;return l&&m&&m.height>0||f&&m&&s(m)?(e===null&&(e=new Co(n)),d=l?e.fromEquirectangular(a):e.fromCubemap(a),d.texture.pmremVersion=a.pmremVersion,t.set(a,d),a.addEventListener("dispose",r),d.texture):null}}}return a}function s(a){let c=0,l=6;for(let f=0;f<l;f++)a[f]!==void 0&&c++;return c===l}function r(a){let c=a.target;c.removeEventListener("dispose",r);let l=t.get(c);l!==void 0&&(t.delete(c),l.dispose())}function o(){t=new WeakMap,e!==null&&(e.dispose(),e=null)}return{get:i,dispose:o}}function V0(n){let t={};function e(i){if(t[i]!==void 0)return t[i];let s;switch(i){case"WEBGL_depth_texture":s=n.getExtension("WEBGL_depth_texture")||n.getExtension("MOZ_WEBGL_depth_texture")||n.getExtension("WEBKIT_WEBGL_depth_texture");break;case"EXT_texture_filter_anisotropic":s=n.getExtension("EXT_texture_filter_anisotropic")||n.getExtension("MOZ_EXT_texture_filter_anisotropic")||n.getExtension("WEBKIT_EXT_texture_filter_anisotropic");break;case"WEBGL_compressed_texture_s3tc":s=n.getExtension("WEBGL_compressed_texture_s3tc")||n.getExtension("MOZ_WEBGL_compressed_texture_s3tc")||n.getExtension("WEBKIT_WEBGL_compressed_texture_s3tc");break;case"WEBGL_compressed_texture_pvrtc":s=n.getExtension("WEBGL_compressed_texture_pvrtc")||n.getExtension("WEBKIT_WEBGL_compressed_texture_pvrtc");break;default:s=n.getExtension(i)}return t[i]=s,s}return{has:function(i){return e(i)!==null},init:function(){e("EXT_color_buffer_float"),e("WEBGL_clip_cull_distance"),e("OES_texture_float_linear"),e("EXT_color_buffer_half_float"),e("WEBGL_multisampled_render_to_texture"),e("WEBGL_render_shared_exponent")},get:function(i){let s=e(i);return s===null&&fo("THREE.WebGLRenderer: "+i+" extension not supported."),s}}}function G0(n,t,e,i){let s={},r=new WeakMap;function o(d){let h=d.target;h.index!==null&&t.remove(h.index);for(let x in h.attributes)t.remove(h.attributes[x]);for(let x in h.morphAttributes){let _=h.morphAttributes[x];for(let u=0,p=_.length;u<p;u++)t.remove(_[u])}h.removeEventListener("dispose",o),delete s[h.id];let m=r.get(h);m&&(t.remove(m),r.delete(h)),i.releaseStatesOfGeometry(h),h.isInstancedBufferGeometry===!0&&delete h._maxInstanceCount,e.memory.geometries--}function a(d,h){return s[h.id]===!0||(h.addEventListener("dispose",o),s[h.id]=!0,e.memory.geometries++),h}function c(d){let h=d.attributes;for(let x in h)t.update(h[x],n.ARRAY_BUFFER);let m=d.morphAttributes;for(let x in m){let _=m[x];for(let u=0,p=_.length;u<p;u++)t.update(_[u],n.ARRAY_BUFFER)}}function l(d){let h=[],m=d.index,x=d.attributes.position,_=0;if(m!==null){let b=m.array;_=m.version;for(let y=0,A=b.length;y<A;y+=3){let R=b[y+0],T=b[y+1],E=b[y+2];h.push(R,T,T,E,E,R)}}else if(x!==void 0){let b=x.array;_=x.version;for(let y=0,A=b.length/3-1;y<A;y+=3){let R=y+0,T=y+1,E=y+2;h.push(R,T,T,E,E,R)}}else return;let u=new(td(h)?wo:So)(h,1);u.version=_;let p=r.get(d);p&&t.remove(p),r.set(d,u)}function f(d){let h=r.get(d);if(h){let m=d.index;m!==null&&h.version<m.version&&l(d)}else l(d);return r.get(d)}return{get:a,update:c,getWireframeAttribute:f}}function W0(n,t,e){let i;function s(h){i=h}let r,o;function a(h){r=h.type,o=h.bytesPerElement}function c(h,m){n.drawElements(i,m,r,h*o),e.update(m,i,1)}function l(h,m,x){x!==0&&(n.drawElementsInstanced(i,m,r,h*o,x),e.update(m,i,x))}function f(h,m,x){if(x===0)return;t.get("WEBGL_multi_draw").multiDrawElementsWEBGL(i,m,0,r,h,0,x);let u=0;for(let p=0;p<x;p++)u+=m[p];e.update(u,i,1)}function d(h,m,x,_){if(x===0)return;let u=t.get("WEBGL_multi_draw");if(u===null)for(let p=0;p<h.length;p++)l(h[p]/o,m[p],_[p]);else{u.multiDrawElementsInstancedWEBGL(i,m,0,r,h,0,_,0,x);let p=0;for(let b=0;b<x;b++)p+=m[b];for(let b=0;b<_.length;b++)e.update(p,i,_[b])}}this.setMode=s,this.setIndex=a,this.render=c,this.renderInstances=l,this.renderMultiDraw=f,this.renderMultiDrawInstances=d}function X0(n){let t={geometries:0,textures:0},e={frame:0,calls:0,triangles:0,points:0,lines:0};function i(r,o,a){switch(e.calls++,o){case n.TRIANGLES:e.triangles+=a*(r/3);break;case n.LINES:e.lines+=a*(r/2);break;case n.LINE_STRIP:e.lines+=a*(r-1);break;case n.LINE_LOOP:e.lines+=a*r;break;case n.POINTS:e.points+=a*r;break;default:console.error("THREE.WebGLInfo: Unknown draw mode:",o);break}}function s(){e.calls=0,e.triangles=0,e.points=0,e.lines=0}return{memory:t,render:e,programs:null,autoReset:!0,reset:s,update:i}}function q0(n,t,e){let i=new WeakMap,s=new ce;function r(o,a,c){let l=o.morphTargetInfluences,f=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,d=f!==void 0?f.length:0,h=i.get(a);if(h===void 0||h.count!==d){let N=function(){E.dispose(),i.delete(a),a.removeEventListener("dispose",N)};h!==void 0&&h.texture.dispose();let m=a.morphAttributes.position!==void 0,x=a.morphAttributes.normal!==void 0,_=a.morphAttributes.color!==void 0,u=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],b=a.morphAttributes.color||[],y=0;m===!0&&(y=1),x===!0&&(y=2),_===!0&&(y=3);let A=a.attributes.position.count*y,R=1;A>t.maxTextureSize&&(R=Math.ceil(A/t.maxTextureSize),A=t.maxTextureSize);let T=new Float32Array(A*R*4*d),E=new Mo(T,A,R,d);E.type=bn,E.needsUpdate=!0;let C=y*4;for(let g=0;g<d;g++){let S=u[g],F=p[g],V=b[g],q=A*R*4*g;for(let et=0;et<S.count;et++){let B=et*C;m===!0&&(s.fromBufferAttribute(S,et),T[q+B+0]=s.x,T[q+B+1]=s.y,T[q+B+2]=s.z,T[q+B+3]=0),x===!0&&(s.fromBufferAttribute(F,et),T[q+B+4]=s.x,T[q+B+5]=s.y,T[q+B+6]=s.z,T[q+B+7]=0),_===!0&&(s.fromBufferAttribute(V,et),T[q+B+8]=s.x,T[q+B+9]=s.y,T[q+B+10]=s.z,T[q+B+11]=V.itemSize===4?s.w:1)}}h={count:d,texture:E,size:new yt(A,R)},i.set(a,h),a.addEventListener("dispose",N)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)c.getUniforms().setValue(n,"morphTexture",o.morphTexture,e);else{let m=0;for(let _=0;_<l.length;_++)m+=l[_];let x=a.morphTargetsRelative?1:1-m;c.getUniforms().setValue(n,"morphTargetBaseInfluence",x),c.getUniforms().setValue(n,"morphTargetInfluences",l)}c.getUniforms().setValue(n,"morphTargetsTexture",h.texture,e),c.getUniforms().setValue(n,"morphTargetsTextureSize",h.size)}return{update:r}}function Y0(n,t,e,i){let s=new WeakMap;function r(c){let l=i.render.frame,f=c.geometry,d=t.get(c,f);if(s.get(d)!==l&&(t.update(d),s.set(d,l)),c.isInstancedMesh&&(c.hasEventListener("dispose",a)===!1&&c.addEventListener("dispose",a),s.get(c)!==l&&(e.update(c.instanceMatrix,n.ARRAY_BUFFER),c.instanceColor!==null&&e.update(c.instanceColor,n.ARRAY_BUFFER),s.set(c,l))),c.isSkinnedMesh){let h=c.skeleton;s.get(h)!==l&&(h.update(),s.set(h,l))}return d}function o(){s=new WeakMap}function a(c){let l=c.target;l.removeEventListener("dispose",a),e.remove(l.instanceMatrix),l.instanceColor!==null&&e.remove(l.instanceColor)}return{update:r,dispose:o}}var Ro=class extends Pe{constructor(t,e,i,s,r,o,a,c,l,f=ps){if(f!==ps&&f!==Ms)throw new Error("DepthTexture format must be either THREE.DepthFormat or THREE.DepthStencilFormat");i===void 0&&f===ps&&(i=Ti),i===void 0&&f===Ms&&(i=ys),super(null,s,r,o,a,c,f,i,l),this.isDepthTexture=!0,this.image={width:t,height:e},this.magFilter=a!==void 0?a:Ee,this.minFilter=c!==void 0?c:Ee,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(t){return super.copy(t),this.compareFunction=t.compareFunction,this}toJSON(t){let e=super.toJSON(t);return this.compareFunction!==null&&(e.compareFunction=this.compareFunction),e}},sd=new Pe,gu=new Ro(1,1),rd=new Mo,od=new rl,ad=new Eo,xu=[],vu=[],_u=new Float32Array(16),yu=new Float32Array(9),Mu=new Float32Array(4);function Ts(n,t,e){let i=n[0];if(i<=0||i>0)return n;let s=t*e,r=xu[s];if(r===void 0&&(r=new Float32Array(s),xu[s]=r),t!==0){i.toArray(r,0);for(let o=1,a=0;o!==t;++o)a+=e,n[o].toArray(r,a)}return r}function ge(n,t){if(n.length!==t.length)return!1;for(let e=0,i=n.length;e<i;e++)if(n[e]!==t[e])return!1;return!0}function xe(n,t){for(let e=0,i=t.length;e<i;e++)n[e]=t[e]}function Go(n,t){let e=vu[t];e===void 0&&(e=new Int32Array(t),vu[t]=e);for(let i=0;i!==t;++i)e[i]=n.allocateTextureUnit();return e}function Z0(n,t){let e=this.cache;e[0]!==t&&(n.uniform1f(this.addr,t),e[0]=t)}function j0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2f(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(ge(e,t))return;n.uniform2fv(this.addr,t),xe(e,t)}}function K0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3f(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else if(t.r!==void 0)(e[0]!==t.r||e[1]!==t.g||e[2]!==t.b)&&(n.uniform3f(this.addr,t.r,t.g,t.b),e[0]=t.r,e[1]=t.g,e[2]=t.b);else{if(ge(e,t))return;n.uniform3fv(this.addr,t),xe(e,t)}}function J0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4f(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(ge(e,t))return;n.uniform4fv(this.addr,t),xe(e,t)}}function Q0(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(ge(e,t))return;n.uniformMatrix2fv(this.addr,!1,t),xe(e,t)}else{if(ge(e,i))return;Mu.set(i),n.uniformMatrix2fv(this.addr,!1,Mu),xe(e,i)}}function $0(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(ge(e,t))return;n.uniformMatrix3fv(this.addr,!1,t),xe(e,t)}else{if(ge(e,i))return;yu.set(i),n.uniformMatrix3fv(this.addr,!1,yu),xe(e,i)}}function tx(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(ge(e,t))return;n.uniformMatrix4fv(this.addr,!1,t),xe(e,t)}else{if(ge(e,i))return;_u.set(i),n.uniformMatrix4fv(this.addr,!1,_u),xe(e,i)}}function ex(n,t){let e=this.cache;e[0]!==t&&(n.uniform1i(this.addr,t),e[0]=t)}function nx(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2i(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(ge(e,t))return;n.uniform2iv(this.addr,t),xe(e,t)}}function ix(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3i(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else{if(ge(e,t))return;n.uniform3iv(this.addr,t),xe(e,t)}}function sx(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4i(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(ge(e,t))return;n.uniform4iv(this.addr,t),xe(e,t)}}function rx(n,t){let e=this.cache;e[0]!==t&&(n.uniform1ui(this.addr,t),e[0]=t)}function ox(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2ui(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(ge(e,t))return;n.uniform2uiv(this.addr,t),xe(e,t)}}function ax(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3ui(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else{if(ge(e,t))return;n.uniform3uiv(this.addr,t),xe(e,t)}}function cx(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4ui(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(ge(e,t))return;n.uniform4uiv(this.addr,t),xe(e,t)}}function lx(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s);let r;this.type===n.SAMPLER_2D_SHADOW?(gu.compareFunction=$u,r=gu):r=sd,e.setTexture2D(t||r,s)}function hx(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTexture3D(t||od,s)}function ux(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTextureCube(t||ad,s)}function dx(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTexture2DArray(t||rd,s)}function fx(n){switch(n){case 5126:return Z0;case 35664:return j0;case 35665:return K0;case 35666:return J0;case 35674:return Q0;case 35675:return $0;case 35676:return tx;case 5124:case 35670:return ex;case 35667:case 35671:return nx;case 35668:case 35672:return ix;case 35669:case 35673:return sx;case 5125:return rx;case 36294:return ox;case 36295:return ax;case 36296:return cx;case 35678:case 36198:case 36298:case 36306:case 35682:return lx;case 35679:case 36299:case 36307:return hx;case 35680:case 36300:case 36308:case 36293:return ux;case 36289:case 36303:case 36311:case 36292:return dx}}function px(n,t){n.uniform1fv(this.addr,t)}function mx(n,t){let e=Ts(t,this.size,2);n.uniform2fv(this.addr,e)}function gx(n,t){let e=Ts(t,this.size,3);n.uniform3fv(this.addr,e)}function xx(n,t){let e=Ts(t,this.size,4);n.uniform4fv(this.addr,e)}function vx(n,t){let e=Ts(t,this.size,4);n.uniformMatrix2fv(this.addr,!1,e)}function _x(n,t){let e=Ts(t,this.size,9);n.uniformMatrix3fv(this.addr,!1,e)}function yx(n,t){let e=Ts(t,this.size,16);n.uniformMatrix4fv(this.addr,!1,e)}function Mx(n,t){n.uniform1iv(this.addr,t)}function bx(n,t){n.uniform2iv(this.addr,t)}function Sx(n,t){n.uniform3iv(this.addr,t)}function wx(n,t){n.uniform4iv(this.addr,t)}function Ax(n,t){n.uniform1uiv(this.addr,t)}function Ex(n,t){n.uniform2uiv(this.addr,t)}function Tx(n,t){n.uniform3uiv(this.addr,t)}function Cx(n,t){n.uniform4uiv(this.addr,t)}function Rx(n,t,e){let i=this.cache,s=t.length,r=Go(e,s);ge(i,r)||(n.uniform1iv(this.addr,r),xe(i,r));for(let o=0;o!==s;++o)e.setTexture2D(t[o]||sd,r[o])}function Px(n,t,e){let i=this.cache,s=t.length,r=Go(e,s);ge(i,r)||(n.uniform1iv(this.addr,r),xe(i,r));for(let o=0;o!==s;++o)e.setTexture3D(t[o]||od,r[o])}function Ix(n,t,e){let i=this.cache,s=t.length,r=Go(e,s);ge(i,r)||(n.uniform1iv(this.addr,r),xe(i,r));for(let o=0;o!==s;++o)e.setTextureCube(t[o]||ad,r[o])}function Lx(n,t,e){let i=this.cache,s=t.length,r=Go(e,s);ge(i,r)||(n.uniform1iv(this.addr,r),xe(i,r));for(let o=0;o!==s;++o)e.setTexture2DArray(t[o]||rd,r[o])}function Dx(n){switch(n){case 5126:return px;case 35664:return mx;case 35665:return gx;case 35666:return xx;case 35674:return vx;case 35675:return _x;case 35676:return yx;case 5124:case 35670:return Mx;case 35667:case 35671:return bx;case 35668:case 35672:return Sx;case 35669:case 35673:return wx;case 5125:return Ax;case 36294:return Ex;case 36295:return Tx;case 36296:return Cx;case 35678:case 36198:case 36298:case 36306:case 35682:return Rx;case 35679:case 36299:case 36307:return Px;case 35680:case 36300:case 36308:case 36293:return Ix;case 36289:case 36303:case 36311:case 36292:return Lx}}var cl=class{constructor(t,e,i){this.id=t,this.addr=i,this.cache=[],this.type=e.type,this.setValue=fx(e.type)}},ll=class{constructor(t,e,i){this.id=t,this.addr=i,this.cache=[],this.type=e.type,this.size=e.size,this.setValue=Dx(e.type)}},hl=class{constructor(t){this.id=t,this.seq=[],this.map={}}setValue(t,e,i){let s=this.seq;for(let r=0,o=s.length;r!==o;++r){let a=s[r];a.setValue(t,e[a.id],i)}}},mc=/(\w+)(\])?(\[|\.)?/g;function bu(n,t){n.seq.push(t),n.map[t.id]=t}function Ux(n,t,e){let i=n.name,s=i.length;for(mc.lastIndex=0;;){let r=mc.exec(i),o=mc.lastIndex,a=r[1],c=r[2]==="]",l=r[3];if(c&&(a=a|0),l===void 0||l==="["&&o+2===s){bu(e,l===void 0?new cl(a,n,t):new ll(a,n,t));break}else{let d=e.map[a];d===void 0&&(d=new hl(a),bu(e,d)),e=d}}}var gs=class{constructor(t,e){this.seq=[],this.map={};let i=t.getProgramParameter(e,t.ACTIVE_UNIFORMS);for(let s=0;s<i;++s){let r=t.getActiveUniform(e,s),o=t.getUniformLocation(e,r.name);Ux(r,o,this)}}setValue(t,e,i,s){let r=this.map[e];r!==void 0&&r.setValue(t,i,s)}setOptional(t,e,i){let s=e[i];s!==void 0&&this.setValue(t,i,s)}static upload(t,e,i,s){for(let r=0,o=e.length;r!==o;++r){let a=e[r],c=i[a.id];c.needsUpdate!==!1&&a.setValue(t,c.value,s)}}static seqWithValue(t,e){let i=[];for(let s=0,r=t.length;s!==r;++s){let o=t[s];o.id in e&&i.push(o)}return i}};function Su(n,t,e){let i=n.createShader(t);return n.shaderSource(i,e),n.compileShader(i),i}var Nx=37297,Ox=0;function Fx(n,t){let e=n.split(`
`),i=[],s=Math.max(t-6,0),r=Math.min(t+6,e.length);for(let o=s;o<r;o++){let a=o+1;i.push(`${a===t?">":" "} ${a}: ${e[o]}`)}return i.join(`
`)}function Bx(n){let t=Jt.getPrimaries(Jt.workingColorSpace),e=Jt.getPrimaries(n),i;switch(t===e?i="":t===xo&&e===go?i="LinearDisplayP3ToLinearSRGB":t===go&&e===xo&&(i="LinearSRGBToLinearDisplayP3"),n){case si:case Vo:return[i,"LinearTransferOETF"];case yn:case Hl:return[i,"sRGBTransferOETF"];default:return console.warn("THREE.WebGLProgram: Unsupported color space:",n),[i,"LinearTransferOETF"]}}function wu(n,t,e){let i=n.getShaderParameter(t,n.COMPILE_STATUS),s=n.getShaderInfoLog(t).trim();if(i&&s==="")return"";let r=/ERROR: 0:(\d+)/.exec(s);if(r){let o=parseInt(r[1]);return e.toUpperCase()+`

`+s+`

`+Fx(n.getShaderSource(t),o)}else return s}function zx(n,t){let e=Bx(t);return`vec4 ${n}( vec4 value ) { return ${e[0]}( ${e[1]}( value ) ); }`}function kx(n,t){let e;switch(t){case Kf:e="Linear";break;case Jf:e="Reinhard";break;case Qf:e="Cineon";break;case $f:e="ACESFilmic";break;case ep:e="AgX";break;case np:e="Neutral";break;case tp:e="Custom";break;default:console.warn("THREE.WebGLProgram: Unsupported toneMapping:",t),e="Linear"}return"vec3 "+n+"( vec3 color ) { return "+e+"ToneMapping( color ); }"}var so=new D;function Hx(){Jt.getLuminanceCoefficients(so);let n=so.x.toFixed(4),t=so.y.toFixed(4),e=so.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${n}, ${t}, ${e} );`,"	return dot( weights, rgb );","}"].join(`
`)}function Vx(n){return[n.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",n.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(ir).join(`
`)}function Gx(n){let t=[];for(let e in n){let i=n[e];i!==!1&&t.push("#define "+e+" "+i)}return t.join(`
`)}function Wx(n,t){let e={},i=n.getProgramParameter(t,n.ACTIVE_ATTRIBUTES);for(let s=0;s<i;s++){let r=n.getActiveAttrib(t,s),o=r.name,a=1;r.type===n.FLOAT_MAT2&&(a=2),r.type===n.FLOAT_MAT3&&(a=3),r.type===n.FLOAT_MAT4&&(a=4),e[o]={type:r.type,location:n.getAttribLocation(t,o),locationSize:a}}return e}function ir(n){return n!==""}function Au(n,t){let e=t.numSpotLightShadows+t.numSpotLightMaps-t.numSpotLightShadowsWithMaps;return n.replace(/NUM_DIR_LIGHTS/g,t.numDirLights).replace(/NUM_SPOT_LIGHTS/g,t.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,t.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,e).replace(/NUM_RECT_AREA_LIGHTS/g,t.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,t.numPointLights).replace(/NUM_HEMI_LIGHTS/g,t.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,t.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,t.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,t.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,t.numPointLightShadows)}function Eu(n,t){return n.replace(/NUM_CLIPPING_PLANES/g,t.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,t.numClippingPlanes-t.numClipIntersection)}var Xx=/^[ \t]*#include +<([\w\d./]+)>/gm;function ul(n){return n.replace(Xx,Yx)}var qx=new Map;function Yx(n,t){let e=Bt[t];if(e===void 0){let i=qx.get(t);if(i!==void 0)e=Bt[i],console.warn('THREE.WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',t,i);else throw new Error("Can not resolve #include <"+t+">")}return ul(e)}var Zx=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Tu(n){return n.replace(Zx,jx)}function jx(n,t,e,i){let s="";for(let r=parseInt(t);r<parseInt(e);r++)s+=i.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return s}function Cu(n){let t=`precision ${n.precision} float;
	precision ${n.precision} int;
	precision ${n.precision} sampler2D;
	precision ${n.precision} samplerCube;
	precision ${n.precision} sampler3D;
	precision ${n.precision} sampler2DArray;
	precision ${n.precision} sampler2DShadow;
	precision ${n.precision} samplerCubeShadow;
	precision ${n.precision} sampler2DArrayShadow;
	precision ${n.precision} isampler2D;
	precision ${n.precision} isampler3D;
	precision ${n.precision} isamplerCube;
	precision ${n.precision} isampler2DArray;
	precision ${n.precision} usampler2D;
	precision ${n.precision} usampler3D;
	precision ${n.precision} usamplerCube;
	precision ${n.precision} usampler2DArray;
	`;return n.precision==="highp"?t+=`
#define HIGH_PRECISION`:n.precision==="mediump"?t+=`
#define MEDIUM_PRECISION`:n.precision==="lowp"&&(t+=`
#define LOW_PRECISION`),t}function Kx(n){let t="SHADOWMAP_TYPE_BASIC";return n.shadowMapType===ku?t="SHADOWMAP_TYPE_PCF":n.shadowMapType===Rf?t="SHADOWMAP_TYPE_PCF_SOFT":n.shadowMapType===Un&&(t="SHADOWMAP_TYPE_VSM"),t}function Jx(n){let t="ENVMAP_TYPE_CUBE";if(n.envMap)switch(n.envMapMode){case vs:case _s:t="ENVMAP_TYPE_CUBE";break;case Ho:t="ENVMAP_TYPE_CUBE_UV";break}return t}function Qx(n){let t="ENVMAP_MODE_REFLECTION";return n.envMap&&n.envMapMode===_s&&(t="ENVMAP_MODE_REFRACTION"),t}function $x(n){let t="ENVMAP_BLENDING_NONE";if(n.envMap)switch(n.combine){case Hu:t="ENVMAP_BLENDING_MULTIPLY";break;case Zf:t="ENVMAP_BLENDING_MIX";break;case jf:t="ENVMAP_BLENDING_ADD";break}return t}function tv(n){let t=n.envMapCubeUVHeight;if(t===null)return null;let e=Math.log2(t)-2,i=1/t;return{texelWidth:1/(3*Math.max(Math.pow(2,e),112)),texelHeight:i,maxMip:e}}function ev(n,t,e,i){let s=n.getContext(),r=e.defines,o=e.vertexShader,a=e.fragmentShader,c=Kx(e),l=Jx(e),f=Qx(e),d=$x(e),h=tv(e),m=Vx(e),x=Gx(r),_=s.createProgram(),u,p,b=e.glslVersion?"#version "+e.glslVersion+`
`:"";e.isRawShaderMaterial?(u=["#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,x].filter(ir).join(`
`),u.length>0&&(u+=`
`),p=["#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,x].filter(ir).join(`
`),p.length>0&&(p+=`
`)):(u=[Cu(e),"#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,x,e.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",e.batching?"#define USE_BATCHING":"",e.batchingColor?"#define USE_BATCHING_COLOR":"",e.instancing?"#define USE_INSTANCING":"",e.instancingColor?"#define USE_INSTANCING_COLOR":"",e.instancingMorph?"#define USE_INSTANCING_MORPH":"",e.useFog&&e.fog?"#define USE_FOG":"",e.useFog&&e.fogExp2?"#define FOG_EXP2":"",e.map?"#define USE_MAP":"",e.envMap?"#define USE_ENVMAP":"",e.envMap?"#define "+f:"",e.lightMap?"#define USE_LIGHTMAP":"",e.aoMap?"#define USE_AOMAP":"",e.bumpMap?"#define USE_BUMPMAP":"",e.normalMap?"#define USE_NORMALMAP":"",e.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",e.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",e.displacementMap?"#define USE_DISPLACEMENTMAP":"",e.emissiveMap?"#define USE_EMISSIVEMAP":"",e.anisotropy?"#define USE_ANISOTROPY":"",e.anisotropyMap?"#define USE_ANISOTROPYMAP":"",e.clearcoatMap?"#define USE_CLEARCOATMAP":"",e.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",e.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",e.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",e.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",e.specularMap?"#define USE_SPECULARMAP":"",e.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",e.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",e.roughnessMap?"#define USE_ROUGHNESSMAP":"",e.metalnessMap?"#define USE_METALNESSMAP":"",e.alphaMap?"#define USE_ALPHAMAP":"",e.alphaHash?"#define USE_ALPHAHASH":"",e.transmission?"#define USE_TRANSMISSION":"",e.transmissionMap?"#define USE_TRANSMISSIONMAP":"",e.thicknessMap?"#define USE_THICKNESSMAP":"",e.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",e.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",e.mapUv?"#define MAP_UV "+e.mapUv:"",e.alphaMapUv?"#define ALPHAMAP_UV "+e.alphaMapUv:"",e.lightMapUv?"#define LIGHTMAP_UV "+e.lightMapUv:"",e.aoMapUv?"#define AOMAP_UV "+e.aoMapUv:"",e.emissiveMapUv?"#define EMISSIVEMAP_UV "+e.emissiveMapUv:"",e.bumpMapUv?"#define BUMPMAP_UV "+e.bumpMapUv:"",e.normalMapUv?"#define NORMALMAP_UV "+e.normalMapUv:"",e.displacementMapUv?"#define DISPLACEMENTMAP_UV "+e.displacementMapUv:"",e.metalnessMapUv?"#define METALNESSMAP_UV "+e.metalnessMapUv:"",e.roughnessMapUv?"#define ROUGHNESSMAP_UV "+e.roughnessMapUv:"",e.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+e.anisotropyMapUv:"",e.clearcoatMapUv?"#define CLEARCOATMAP_UV "+e.clearcoatMapUv:"",e.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+e.clearcoatNormalMapUv:"",e.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+e.clearcoatRoughnessMapUv:"",e.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+e.iridescenceMapUv:"",e.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+e.iridescenceThicknessMapUv:"",e.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+e.sheenColorMapUv:"",e.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+e.sheenRoughnessMapUv:"",e.specularMapUv?"#define SPECULARMAP_UV "+e.specularMapUv:"",e.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+e.specularColorMapUv:"",e.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+e.specularIntensityMapUv:"",e.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+e.transmissionMapUv:"",e.thicknessMapUv?"#define THICKNESSMAP_UV "+e.thicknessMapUv:"",e.vertexTangents&&e.flatShading===!1?"#define USE_TANGENT":"",e.vertexColors?"#define USE_COLOR":"",e.vertexAlphas?"#define USE_COLOR_ALPHA":"",e.vertexUv1s?"#define USE_UV1":"",e.vertexUv2s?"#define USE_UV2":"",e.vertexUv3s?"#define USE_UV3":"",e.pointsUvs?"#define USE_POINTS_UV":"",e.flatShading?"#define FLAT_SHADED":"",e.skinning?"#define USE_SKINNING":"",e.morphTargets?"#define USE_MORPHTARGETS":"",e.morphNormals&&e.flatShading===!1?"#define USE_MORPHNORMALS":"",e.morphColors?"#define USE_MORPHCOLORS":"",e.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+e.morphTextureStride:"",e.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+e.morphTargetsCount:"",e.doubleSided?"#define DOUBLE_SIDED":"",e.flipSided?"#define FLIP_SIDED":"",e.shadowMapEnabled?"#define USE_SHADOWMAP":"",e.shadowMapEnabled?"#define "+c:"",e.sizeAttenuation?"#define USE_SIZEATTENUATION":"",e.numLightProbes>0?"#define USE_LIGHT_PROBES":"",e.logarithmicDepthBuffer?"#define USE_LOGDEPTHBUF":"",e.reverseDepthBuffer?"#define USE_REVERSEDEPTHBUF":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(ir).join(`
`),p=[Cu(e),"#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,x,e.useFog&&e.fog?"#define USE_FOG":"",e.useFog&&e.fogExp2?"#define FOG_EXP2":"",e.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",e.map?"#define USE_MAP":"",e.matcap?"#define USE_MATCAP":"",e.envMap?"#define USE_ENVMAP":"",e.envMap?"#define "+l:"",e.envMap?"#define "+f:"",e.envMap?"#define "+d:"",h?"#define CUBEUV_TEXEL_WIDTH "+h.texelWidth:"",h?"#define CUBEUV_TEXEL_HEIGHT "+h.texelHeight:"",h?"#define CUBEUV_MAX_MIP "+h.maxMip+".0":"",e.lightMap?"#define USE_LIGHTMAP":"",e.aoMap?"#define USE_AOMAP":"",e.bumpMap?"#define USE_BUMPMAP":"",e.normalMap?"#define USE_NORMALMAP":"",e.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",e.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",e.emissiveMap?"#define USE_EMISSIVEMAP":"",e.anisotropy?"#define USE_ANISOTROPY":"",e.anisotropyMap?"#define USE_ANISOTROPYMAP":"",e.clearcoat?"#define USE_CLEARCOAT":"",e.clearcoatMap?"#define USE_CLEARCOATMAP":"",e.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",e.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",e.dispersion?"#define USE_DISPERSION":"",e.iridescence?"#define USE_IRIDESCENCE":"",e.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",e.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",e.specularMap?"#define USE_SPECULARMAP":"",e.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",e.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",e.roughnessMap?"#define USE_ROUGHNESSMAP":"",e.metalnessMap?"#define USE_METALNESSMAP":"",e.alphaMap?"#define USE_ALPHAMAP":"",e.alphaTest?"#define USE_ALPHATEST":"",e.alphaHash?"#define USE_ALPHAHASH":"",e.sheen?"#define USE_SHEEN":"",e.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",e.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",e.transmission?"#define USE_TRANSMISSION":"",e.transmissionMap?"#define USE_TRANSMISSIONMAP":"",e.thicknessMap?"#define USE_THICKNESSMAP":"",e.vertexTangents&&e.flatShading===!1?"#define USE_TANGENT":"",e.vertexColors||e.instancingColor||e.batchingColor?"#define USE_COLOR":"",e.vertexAlphas?"#define USE_COLOR_ALPHA":"",e.vertexUv1s?"#define USE_UV1":"",e.vertexUv2s?"#define USE_UV2":"",e.vertexUv3s?"#define USE_UV3":"",e.pointsUvs?"#define USE_POINTS_UV":"",e.gradientMap?"#define USE_GRADIENTMAP":"",e.flatShading?"#define FLAT_SHADED":"",e.doubleSided?"#define DOUBLE_SIDED":"",e.flipSided?"#define FLIP_SIDED":"",e.shadowMapEnabled?"#define USE_SHADOWMAP":"",e.shadowMapEnabled?"#define "+c:"",e.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",e.numLightProbes>0?"#define USE_LIGHT_PROBES":"",e.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",e.logarithmicDepthBuffer?"#define USE_LOGDEPTHBUF":"",e.reverseDepthBuffer?"#define USE_REVERSEDEPTHBUF":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",e.toneMapping!==ti?"#define TONE_MAPPING":"",e.toneMapping!==ti?Bt.tonemapping_pars_fragment:"",e.toneMapping!==ti?kx("toneMapping",e.toneMapping):"",e.dithering?"#define DITHERING":"",e.opaque?"#define OPAQUE":"",Bt.colorspace_pars_fragment,zx("linearToOutputTexel",e.outputColorSpace),Hx(),e.useDepthPacking?"#define DEPTH_PACKING "+e.depthPacking:"",`
`].filter(ir).join(`
`)),o=ul(o),o=Au(o,e),o=Eu(o,e),a=ul(a),a=Au(a,e),a=Eu(a,e),o=Tu(o),a=Tu(a),e.isRawShaderMaterial!==!0&&(b=`#version 300 es
`,u=[m,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+u,p=["#define varying in",e.glslVersion===Xh?"":"layout(location = 0) out highp vec4 pc_fragColor;",e.glslVersion===Xh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let y=b+u+o,A=b+p+a,R=Su(s,s.VERTEX_SHADER,y),T=Su(s,s.FRAGMENT_SHADER,A);s.attachShader(_,R),s.attachShader(_,T),e.index0AttributeName!==void 0?s.bindAttribLocation(_,0,e.index0AttributeName):e.morphTargets===!0&&s.bindAttribLocation(_,0,"position"),s.linkProgram(_);function E(S){if(n.debug.checkShaderErrors){let F=s.getProgramInfoLog(_).trim(),V=s.getShaderInfoLog(R).trim(),q=s.getShaderInfoLog(T).trim(),et=!0,B=!0;if(s.getProgramParameter(_,s.LINK_STATUS)===!1)if(et=!1,typeof n.debug.onShaderError=="function")n.debug.onShaderError(s,_,R,T);else{let it=wu(s,R,"vertex"),Z=wu(s,T,"fragment");console.error("THREE.WebGLProgram: Shader Error "+s.getError()+" - VALIDATE_STATUS "+s.getProgramParameter(_,s.VALIDATE_STATUS)+`

Material Name: `+S.name+`
Material Type: `+S.type+`

Program Info Log: `+F+`
`+it+`
`+Z)}else F!==""?console.warn("THREE.WebGLProgram: Program Info Log:",F):(V===""||q==="")&&(B=!1);B&&(S.diagnostics={runnable:et,programLog:F,vertexShader:{log:V,prefix:u},fragmentShader:{log:q,prefix:p}})}s.deleteShader(R),s.deleteShader(T),C=new gs(s,_),N=Wx(s,_)}let C;this.getUniforms=function(){return C===void 0&&E(this),C};let N;this.getAttributes=function(){return N===void 0&&E(this),N};let g=e.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return g===!1&&(g=s.getProgramParameter(_,Nx)),g},this.destroy=function(){i.releaseStatesOfProgram(this),s.deleteProgram(_),this.program=void 0},this.type=e.shaderType,this.name=e.shaderName,this.id=Ox++,this.cacheKey=t,this.usedTimes=1,this.program=_,this.vertexShader=R,this.fragmentShader=T,this}var nv=0,dl=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(t){let e=t.vertexShader,i=t.fragmentShader,s=this._getShaderStage(e),r=this._getShaderStage(i),o=this._getShaderCacheForMaterial(t);return o.has(s)===!1&&(o.add(s),s.usedTimes++),o.has(r)===!1&&(o.add(r),r.usedTimes++),this}remove(t){let e=this.materialCache.get(t);for(let i of e)i.usedTimes--,i.usedTimes===0&&this.shaderCache.delete(i.code);return this.materialCache.delete(t),this}getVertexShaderID(t){return this._getShaderStage(t.vertexShader).id}getFragmentShaderID(t){return this._getShaderStage(t.fragmentShader).id}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(t){let e=this.materialCache,i=e.get(t);return i===void 0&&(i=new Set,e.set(t,i)),i}_getShaderStage(t){let e=this.shaderCache,i=e.get(t);return i===void 0&&(i=new fl(t),e.set(t,i)),i}},fl=class{constructor(t){this.id=nv++,this.code=t,this.usedTimes=0}};function iv(n,t,e,i,s,r,o){let a=new lr,c=new dl,l=new Set,f=[],d=s.logarithmicDepthBuffer,h=s.reverseDepthBuffer,m=s.vertexTextures,x=s.precision,_={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distanceRGBA",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function u(g){return l.add(g),g===0?"uv":`uv${g}`}function p(g,S,F,V,q){let et=V.fog,B=q.geometry,it=g.isMeshStandardMaterial?V.environment:null,Z=(g.isMeshStandardMaterial?e:t).get(g.envMap||it),gt=Z&&Z.mapping===Ho?Z.image.height:null,_t=_[g.type];g.precision!==null&&(x=s.getMaxPrecision(g.precision),x!==g.precision&&console.warn("THREE.WebGLProgram.getParameters:",g.precision,"not supported, using",x,"instead."));let bt=B.morphAttributes.position||B.morphAttributes.normal||B.morphAttributes.color,Gt=bt!==void 0?bt.length:0,Wt=0;B.morphAttributes.position!==void 0&&(Wt=1),B.morphAttributes.normal!==void 0&&(Wt=2),B.morphAttributes.color!==void 0&&(Wt=3);let J,st,Mt,xt;if(_t){let Be=Mn[_t];J=Be.vertexShader,st=Be.fragmentShader}else J=g.vertexShader,st=g.fragmentShader,c.update(g),Mt=c.getVertexShaderID(g),xt=c.getFragmentShaderID(g);let Nt=n.getRenderTarget(),It=q.isInstancedMesh===!0,kt=q.isBatchedMesh===!0,Zt=!!g.map,Vt=!!g.matcap,I=!!Z,me=!!g.aoMap,j=!!g.lightMap,ct=!!g.bumpMap,ut=!!g.normalMap,Rt=!!g.displacementMap,z=!!g.emissiveMap,M=!!g.metalnessMap,v=!!g.roughnessMap,P=g.anisotropy>0,H=g.clearcoat>0,K=g.dispersion>0,k=g.iridescence>0,nt=g.sheen>0,Q=g.transmission>0,X=P&&!!g.anisotropyMap,ht=H&&!!g.clearcoatMap,tt=H&&!!g.clearcoatNormalMap,lt=H&&!!g.clearcoatRoughnessMap,At=k&&!!g.iridescenceMap,rt=k&&!!g.iridescenceThicknessMap,ot=nt&&!!g.sheenColorMap,Lt=nt&&!!g.sheenRoughnessMap,wt=!!g.specularMap,Ft=!!g.specularColorMap,L=!!g.specularIntensityMap,ft=Q&&!!g.transmissionMap,Y=Q&&!!g.thicknessMap,$=!!g.gradientMap,pt=!!g.alphaMap,vt=g.alphaTest>0,Xt=!!g.alphaHash,ue=!!g.extensions,Fe=ti;g.toneMapped&&(Nt===null||Nt.isXRRenderTarget===!0)&&(Fe=n.toneMapping);let qt={shaderID:_t,shaderType:g.type,shaderName:g.name,vertexShader:J,fragmentShader:st,defines:g.defines,customVertexShaderID:Mt,customFragmentShaderID:xt,isRawShaderMaterial:g.isRawShaderMaterial===!0,glslVersion:g.glslVersion,precision:x,batching:kt,batchingColor:kt&&q._colorsTexture!==null,instancing:It,instancingColor:It&&q.instanceColor!==null,instancingMorph:It&&q.morphTexture!==null,supportsVertexTextures:m,outputColorSpace:Nt===null?n.outputColorSpace:Nt.isXRRenderTarget===!0?Nt.texture.colorSpace:si,alphaToCoverage:!!g.alphaToCoverage,map:Zt,matcap:Vt,envMap:I,envMapMode:I&&Z.mapping,envMapCubeUVHeight:gt,aoMap:me,lightMap:j,bumpMap:ct,normalMap:ut,displacementMap:m&&Rt,emissiveMap:z,normalMapObjectSpace:ut&&g.normalMapType===op,normalMapTangentSpace:ut&&g.normalMapType===Qu,metalnessMap:M,roughnessMap:v,anisotropy:P,anisotropyMap:X,clearcoat:H,clearcoatMap:ht,clearcoatNormalMap:tt,clearcoatRoughnessMap:lt,dispersion:K,iridescence:k,iridescenceMap:At,iridescenceThicknessMap:rt,sheen:nt,sheenColorMap:ot,sheenRoughnessMap:Lt,specularMap:wt,specularColorMap:Ft,specularIntensityMap:L,transmission:Q,transmissionMap:ft,thicknessMap:Y,gradientMap:$,opaque:g.transparent===!1&&g.blending===fs&&g.alphaToCoverage===!1,alphaMap:pt,alphaTest:vt,alphaHash:Xt,combine:g.combine,mapUv:Zt&&u(g.map.channel),aoMapUv:me&&u(g.aoMap.channel),lightMapUv:j&&u(g.lightMap.channel),bumpMapUv:ct&&u(g.bumpMap.channel),normalMapUv:ut&&u(g.normalMap.channel),displacementMapUv:Rt&&u(g.displacementMap.channel),emissiveMapUv:z&&u(g.emissiveMap.channel),metalnessMapUv:M&&u(g.metalnessMap.channel),roughnessMapUv:v&&u(g.roughnessMap.channel),anisotropyMapUv:X&&u(g.anisotropyMap.channel),clearcoatMapUv:ht&&u(g.clearcoatMap.channel),clearcoatNormalMapUv:tt&&u(g.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:lt&&u(g.clearcoatRoughnessMap.channel),iridescenceMapUv:At&&u(g.iridescenceMap.channel),iridescenceThicknessMapUv:rt&&u(g.iridescenceThicknessMap.channel),sheenColorMapUv:ot&&u(g.sheenColorMap.channel),sheenRoughnessMapUv:Lt&&u(g.sheenRoughnessMap.channel),specularMapUv:wt&&u(g.specularMap.channel),specularColorMapUv:Ft&&u(g.specularColorMap.channel),specularIntensityMapUv:L&&u(g.specularIntensityMap.channel),transmissionMapUv:ft&&u(g.transmissionMap.channel),thicknessMapUv:Y&&u(g.thicknessMap.channel),alphaMapUv:pt&&u(g.alphaMap.channel),vertexTangents:!!B.attributes.tangent&&(ut||P),vertexColors:g.vertexColors,vertexAlphas:g.vertexColors===!0&&!!B.attributes.color&&B.attributes.color.itemSize===4,pointsUvs:q.isPoints===!0&&!!B.attributes.uv&&(Zt||pt),fog:!!et,useFog:g.fog===!0,fogExp2:!!et&&et.isFogExp2,flatShading:g.flatShading===!0,sizeAttenuation:g.sizeAttenuation===!0,logarithmicDepthBuffer:d,reverseDepthBuffer:h,skinning:q.isSkinnedMesh===!0,morphTargets:B.morphAttributes.position!==void 0,morphNormals:B.morphAttributes.normal!==void 0,morphColors:B.morphAttributes.color!==void 0,morphTargetsCount:Gt,morphTextureStride:Wt,numDirLights:S.directional.length,numPointLights:S.point.length,numSpotLights:S.spot.length,numSpotLightMaps:S.spotLightMap.length,numRectAreaLights:S.rectArea.length,numHemiLights:S.hemi.length,numDirLightShadows:S.directionalShadowMap.length,numPointLightShadows:S.pointShadowMap.length,numSpotLightShadows:S.spotShadowMap.length,numSpotLightShadowsWithMaps:S.numSpotLightShadowsWithMaps,numLightProbes:S.numLightProbes,numClippingPlanes:o.numPlanes,numClipIntersection:o.numIntersection,dithering:g.dithering,shadowMapEnabled:n.shadowMap.enabled&&F.length>0,shadowMapType:n.shadowMap.type,toneMapping:Fe,decodeVideoTexture:Zt&&g.map.isVideoTexture===!0&&Jt.getTransfer(g.map.colorSpace)===ie,premultipliedAlpha:g.premultipliedAlpha,doubleSided:g.side===Nn,flipSided:g.side===ze,useDepthPacking:g.depthPacking>=0,depthPacking:g.depthPacking||0,index0AttributeName:g.index0AttributeName,extensionClipCullDistance:ue&&g.extensions.clipCullDistance===!0&&i.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(ue&&g.extensions.multiDraw===!0||kt)&&i.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:i.has("KHR_parallel_shader_compile"),customProgramCacheKey:g.customProgramCacheKey()};return qt.vertexUv1s=l.has(1),qt.vertexUv2s=l.has(2),qt.vertexUv3s=l.has(3),l.clear(),qt}function b(g){let S=[];if(g.shaderID?S.push(g.shaderID):(S.push(g.customVertexShaderID),S.push(g.customFragmentShaderID)),g.defines!==void 0)for(let F in g.defines)S.push(F),S.push(g.defines[F]);return g.isRawShaderMaterial===!1&&(y(S,g),A(S,g),S.push(n.outputColorSpace)),S.push(g.customProgramCacheKey),S.join()}function y(g,S){g.push(S.precision),g.push(S.outputColorSpace),g.push(S.envMapMode),g.push(S.envMapCubeUVHeight),g.push(S.mapUv),g.push(S.alphaMapUv),g.push(S.lightMapUv),g.push(S.aoMapUv),g.push(S.bumpMapUv),g.push(S.normalMapUv),g.push(S.displacementMapUv),g.push(S.emissiveMapUv),g.push(S.metalnessMapUv),g.push(S.roughnessMapUv),g.push(S.anisotropyMapUv),g.push(S.clearcoatMapUv),g.push(S.clearcoatNormalMapUv),g.push(S.clearcoatRoughnessMapUv),g.push(S.iridescenceMapUv),g.push(S.iridescenceThicknessMapUv),g.push(S.sheenColorMapUv),g.push(S.sheenRoughnessMapUv),g.push(S.specularMapUv),g.push(S.specularColorMapUv),g.push(S.specularIntensityMapUv),g.push(S.transmissionMapUv),g.push(S.thicknessMapUv),g.push(S.combine),g.push(S.fogExp2),g.push(S.sizeAttenuation),g.push(S.morphTargetsCount),g.push(S.morphAttributeCount),g.push(S.numDirLights),g.push(S.numPointLights),g.push(S.numSpotLights),g.push(S.numSpotLightMaps),g.push(S.numHemiLights),g.push(S.numRectAreaLights),g.push(S.numDirLightShadows),g.push(S.numPointLightShadows),g.push(S.numSpotLightShadows),g.push(S.numSpotLightShadowsWithMaps),g.push(S.numLightProbes),g.push(S.shadowMapType),g.push(S.toneMapping),g.push(S.numClippingPlanes),g.push(S.numClipIntersection),g.push(S.depthPacking)}function A(g,S){a.disableAll(),S.supportsVertexTextures&&a.enable(0),S.instancing&&a.enable(1),S.instancingColor&&a.enable(2),S.instancingMorph&&a.enable(3),S.matcap&&a.enable(4),S.envMap&&a.enable(5),S.normalMapObjectSpace&&a.enable(6),S.normalMapTangentSpace&&a.enable(7),S.clearcoat&&a.enable(8),S.iridescence&&a.enable(9),S.alphaTest&&a.enable(10),S.vertexColors&&a.enable(11),S.vertexAlphas&&a.enable(12),S.vertexUv1s&&a.enable(13),S.vertexUv2s&&a.enable(14),S.vertexUv3s&&a.enable(15),S.vertexTangents&&a.enable(16),S.anisotropy&&a.enable(17),S.alphaHash&&a.enable(18),S.batching&&a.enable(19),S.dispersion&&a.enable(20),S.batchingColor&&a.enable(21),g.push(a.mask),a.disableAll(),S.fog&&a.enable(0),S.useFog&&a.enable(1),S.flatShading&&a.enable(2),S.logarithmicDepthBuffer&&a.enable(3),S.reverseDepthBuffer&&a.enable(4),S.skinning&&a.enable(5),S.morphTargets&&a.enable(6),S.morphNormals&&a.enable(7),S.morphColors&&a.enable(8),S.premultipliedAlpha&&a.enable(9),S.shadowMapEnabled&&a.enable(10),S.doubleSided&&a.enable(11),S.flipSided&&a.enable(12),S.useDepthPacking&&a.enable(13),S.dithering&&a.enable(14),S.transmission&&a.enable(15),S.sheen&&a.enable(16),S.opaque&&a.enable(17),S.pointsUvs&&a.enable(18),S.decodeVideoTexture&&a.enable(19),S.alphaToCoverage&&a.enable(20),g.push(a.mask)}function R(g){let S=_[g.type],F;if(S){let V=Mn[S];F=wn.clone(V.uniforms)}else F=g.uniforms;return F}function T(g,S){let F;for(let V=0,q=f.length;V<q;V++){let et=f[V];if(et.cacheKey===S){F=et,++F.usedTimes;break}}return F===void 0&&(F=new ev(n,S,g,r),f.push(F)),F}function E(g){if(--g.usedTimes===0){let S=f.indexOf(g);f[S]=f[f.length-1],f.pop(),g.destroy()}}function C(g){c.remove(g)}function N(){c.dispose()}return{getParameters:p,getProgramCacheKey:b,getUniforms:R,acquireProgram:T,releaseProgram:E,releaseShaderCache:C,programs:f,dispose:N}}function sv(){let n=new WeakMap;function t(o){return n.has(o)}function e(o){let a=n.get(o);return a===void 0&&(a={},n.set(o,a)),a}function i(o){n.delete(o)}function s(o,a,c){n.get(o)[a]=c}function r(){n=new WeakMap}return{has:t,get:e,remove:i,update:s,dispose:r}}function rv(n,t){return n.groupOrder!==t.groupOrder?n.groupOrder-t.groupOrder:n.renderOrder!==t.renderOrder?n.renderOrder-t.renderOrder:n.material.id!==t.material.id?n.material.id-t.material.id:n.z!==t.z?n.z-t.z:n.id-t.id}function Ru(n,t){return n.groupOrder!==t.groupOrder?n.groupOrder-t.groupOrder:n.renderOrder!==t.renderOrder?n.renderOrder-t.renderOrder:n.z!==t.z?t.z-n.z:n.id-t.id}function Pu(){let n=[],t=0,e=[],i=[],s=[];function r(){t=0,e.length=0,i.length=0,s.length=0}function o(d,h,m,x,_,u){let p=n[t];return p===void 0?(p={id:d.id,object:d,geometry:h,material:m,groupOrder:x,renderOrder:d.renderOrder,z:_,group:u},n[t]=p):(p.id=d.id,p.object=d,p.geometry=h,p.material=m,p.groupOrder=x,p.renderOrder=d.renderOrder,p.z=_,p.group=u),t++,p}function a(d,h,m,x,_,u){let p=o(d,h,m,x,_,u);m.transmission>0?i.push(p):m.transparent===!0?s.push(p):e.push(p)}function c(d,h,m,x,_,u){let p=o(d,h,m,x,_,u);m.transmission>0?i.unshift(p):m.transparent===!0?s.unshift(p):e.unshift(p)}function l(d,h){e.length>1&&e.sort(d||rv),i.length>1&&i.sort(h||Ru),s.length>1&&s.sort(h||Ru)}function f(){for(let d=t,h=n.length;d<h;d++){let m=n[d];if(m.id===null)break;m.id=null,m.object=null,m.geometry=null,m.material=null,m.group=null}}return{opaque:e,transmissive:i,transparent:s,init:r,push:a,unshift:c,finish:f,sort:l}}function ov(){let n=new WeakMap;function t(i,s){let r=n.get(i),o;return r===void 0?(o=new Pu,n.set(i,[o])):s>=r.length?(o=new Pu,r.push(o)):o=r[s],o}function e(){n=new WeakMap}return{get:t,dispose:e}}function av(){let n={};return{get:function(t){if(n[t.id]!==void 0)return n[t.id];let e;switch(t.type){case"DirectionalLight":e={direction:new D,color:new Ot};break;case"SpotLight":e={position:new D,direction:new D,color:new Ot,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":e={position:new D,color:new Ot,distance:0,decay:0};break;case"HemisphereLight":e={direction:new D,skyColor:new Ot,groundColor:new Ot};break;case"RectAreaLight":e={color:new Ot,position:new D,halfWidth:new D,halfHeight:new D};break}return n[t.id]=e,e}}}function cv(){let n={};return{get:function(t){if(n[t.id]!==void 0)return n[t.id];let e;switch(t.type){case"DirectionalLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt};break;case"SpotLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt};break;case"PointLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt,shadowCameraNear:1,shadowCameraFar:1e3};break}return n[t.id]=e,e}}}var lv=0;function hv(n,t){return(t.castShadow?2:0)-(n.castShadow?2:0)+(t.map?1:0)-(n.map?1:0)}function uv(n){let t=new av,e=cv(),i={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let l=0;l<9;l++)i.probe.push(new D);let s=new D,r=new $t,o=new $t;function a(l){let f=0,d=0,h=0;for(let N=0;N<9;N++)i.probe[N].set(0,0,0);let m=0,x=0,_=0,u=0,p=0,b=0,y=0,A=0,R=0,T=0,E=0;l.sort(hv);for(let N=0,g=l.length;N<g;N++){let S=l[N],F=S.color,V=S.intensity,q=S.distance,et=S.shadow&&S.shadow.map?S.shadow.map.texture:null;if(S.isAmbientLight)f+=F.r*V,d+=F.g*V,h+=F.b*V;else if(S.isLightProbe){for(let B=0;B<9;B++)i.probe[B].addScaledVector(S.sh.coefficients[B],V);E++}else if(S.isDirectionalLight){let B=t.get(S);if(B.color.copy(S.color).multiplyScalar(S.intensity),S.castShadow){let it=S.shadow,Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,i.directionalShadow[m]=Z,i.directionalShadowMap[m]=et,i.directionalShadowMatrix[m]=S.shadow.matrix,b++}i.directional[m]=B,m++}else if(S.isSpotLight){let B=t.get(S);B.position.setFromMatrixPosition(S.matrixWorld),B.color.copy(F).multiplyScalar(V),B.distance=q,B.coneCos=Math.cos(S.angle),B.penumbraCos=Math.cos(S.angle*(1-S.penumbra)),B.decay=S.decay,i.spot[_]=B;let it=S.shadow;if(S.map&&(i.spotLightMap[R]=S.map,R++,it.updateMatrices(S),S.castShadow&&T++),i.spotLightMatrix[_]=it.matrix,S.castShadow){let Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,i.spotShadow[_]=Z,i.spotShadowMap[_]=et,A++}_++}else if(S.isRectAreaLight){let B=t.get(S);B.color.copy(F).multiplyScalar(V),B.halfWidth.set(S.width*.5,0,0),B.halfHeight.set(0,S.height*.5,0),i.rectArea[u]=B,u++}else if(S.isPointLight){let B=t.get(S);if(B.color.copy(S.color).multiplyScalar(S.intensity),B.distance=S.distance,B.decay=S.decay,S.castShadow){let it=S.shadow,Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,Z.shadowCameraNear=it.camera.near,Z.shadowCameraFar=it.camera.far,i.pointShadow[x]=Z,i.pointShadowMap[x]=et,i.pointShadowMatrix[x]=S.shadow.matrix,y++}i.point[x]=B,x++}else if(S.isHemisphereLight){let B=t.get(S);B.skyColor.copy(S.color).multiplyScalar(V),B.groundColor.copy(S.groundColor).multiplyScalar(V),i.hemi[p]=B,p++}}u>0&&(n.has("OES_texture_float_linear")===!0?(i.rectAreaLTC1=dt.LTC_FLOAT_1,i.rectAreaLTC2=dt.LTC_FLOAT_2):(i.rectAreaLTC1=dt.LTC_HALF_1,i.rectAreaLTC2=dt.LTC_HALF_2)),i.ambient[0]=f,i.ambient[1]=d,i.ambient[2]=h;let C=i.hash;(C.directionalLength!==m||C.pointLength!==x||C.spotLength!==_||C.rectAreaLength!==u||C.hemiLength!==p||C.numDirectionalShadows!==b||C.numPointShadows!==y||C.numSpotShadows!==A||C.numSpotMaps!==R||C.numLightProbes!==E)&&(i.directional.length=m,i.spot.length=_,i.rectArea.length=u,i.point.length=x,i.hemi.length=p,i.directionalShadow.length=b,i.directionalShadowMap.length=b,i.pointShadow.length=y,i.pointShadowMap.length=y,i.spotShadow.length=A,i.spotShadowMap.length=A,i.directionalShadowMatrix.length=b,i.pointShadowMatrix.length=y,i.spotLightMatrix.length=A+R-T,i.spotLightMap.length=R,i.numSpotLightShadowsWithMaps=T,i.numLightProbes=E,C.directionalLength=m,C.pointLength=x,C.spotLength=_,C.rectAreaLength=u,C.hemiLength=p,C.numDirectionalShadows=b,C.numPointShadows=y,C.numSpotShadows=A,C.numSpotMaps=R,C.numLightProbes=E,i.version=lv++)}function c(l,f){let d=0,h=0,m=0,x=0,_=0,u=f.matrixWorldInverse;for(let p=0,b=l.length;p<b;p++){let y=l[p];if(y.isDirectionalLight){let A=i.directional[d];A.direction.setFromMatrixPosition(y.matrixWorld),s.setFromMatrixPosition(y.target.matrixWorld),A.direction.sub(s),A.direction.transformDirection(u),d++}else if(y.isSpotLight){let A=i.spot[m];A.position.setFromMatrixPosition(y.matrixWorld),A.position.applyMatrix4(u),A.direction.setFromMatrixPosition(y.matrixWorld),s.setFromMatrixPosition(y.target.matrixWorld),A.direction.sub(s),A.direction.transformDirection(u),m++}else if(y.isRectAreaLight){let A=i.rectArea[x];A.position.setFromMatrixPosition(y.matrixWorld),A.position.applyMatrix4(u),o.identity(),r.copy(y.matrixWorld),r.premultiply(u),o.extractRotation(r),A.halfWidth.set(y.width*.5,0,0),A.halfHeight.set(0,y.height*.5,0),A.halfWidth.applyMatrix4(o),A.halfHeight.applyMatrix4(o),x++}else if(y.isPointLight){let A=i.point[h];A.position.setFromMatrixPosition(y.matrixWorld),A.position.applyMatrix4(u),h++}else if(y.isHemisphereLight){let A=i.hemi[_];A.direction.setFromMatrixPosition(y.matrixWorld),A.direction.transformDirection(u),_++}}}return{setup:a,setupView:c,state:i}}function Iu(n){let t=new uv(n),e=[],i=[];function s(f){l.camera=f,e.length=0,i.length=0}function r(f){e.push(f)}function o(f){i.push(f)}function a(){t.setup(e)}function c(f){t.setupView(e,f)}let l={lightsArray:e,shadowsArray:i,camera:null,lights:t,transmissionRenderTarget:{}};return{init:s,state:l,setupLights:a,setupLightsView:c,pushLight:r,pushShadow:o}}function dv(n){let t=new WeakMap;function e(s,r=0){let o=t.get(s),a;return o===void 0?(a=new Iu(n),t.set(s,[a])):r>=o.length?(a=new Iu(n),o.push(a)):a=o[r],a}function i(){t=new WeakMap}return{get:e,dispose:i}}var pl=class extends Ri{constructor(t){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=sp,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(t)}copy(t){return super.copy(t),this.depthPacking=t.depthPacking,this.map=t.map,this.alphaMap=t.alphaMap,this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this}},ml=class extends Ri{constructor(t){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(t)}copy(t){return super.copy(t),this.map=t.map,this.alphaMap=t.alphaMap,this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this}},fv=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,pv=`uniform sampler2D shadow_pass;
uniform vec2 resolution;
uniform float radius;
#include <packing>
void main() {
	const float samples = float( VSM_SAMPLES );
	float mean = 0.0;
	float squared_mean = 0.0;
	float uvStride = samples <= 1.0 ? 0.0 : 2.0 / ( samples - 1.0 );
	float uvStart = samples <= 1.0 ? 0.0 : - 1.0;
	for ( float i = 0.0; i < samples; i ++ ) {
		float uvOffset = uvStart + i * uvStride;
		#ifdef HORIZONTAL_PASS
			vec2 distribution = unpackRGBATo2Half( texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( uvOffset, 0.0 ) * radius ) / resolution ) );
			mean += distribution.x;
			squared_mean += distribution.y * distribution.y + distribution.x * distribution.x;
		#else
			float depth = unpackRGBAToDepth( texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( 0.0, uvOffset ) * radius ) / resolution ) );
			mean += depth;
			squared_mean += depth * depth;
		#endif
	}
	mean = mean / samples;
	squared_mean = squared_mean / samples;
	float std_dev = sqrt( squared_mean - mean * mean );
	gl_FragColor = pack2HalfToRGBA( vec2( mean, std_dev ) );
}`;function mv(n,t,e){let i=new ur,s=new yt,r=new yt,o=new ce,a=new pl({depthPacking:rp}),c=new ml,l={},f=e.maxTextureSize,d={[ei]:ze,[ze]:ei,[Nn]:Nn},h=new te({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new yt},radius:{value:4}},vertexShader:fv,fragmentShader:pv}),m=h.clone();m.defines.HORIZONTAL_PASS=1;let x=new pn;x.setAttribute("position",new Ke(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let _=new Oe(x,h),u=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=ku;let p=this.type;this.render=function(T,E,C){if(u.enabled===!1||u.autoUpdate===!1&&u.needsUpdate===!1||T.length===0)return;let N=n.getRenderTarget(),g=n.getActiveCubeFace(),S=n.getActiveMipmapLevel(),F=n.state;F.setBlending(Sn),F.buffers.color.setClear(1,1,1,1),F.buffers.depth.setTest(!0),F.setScissorTest(!1);let V=p!==Un&&this.type===Un,q=p===Un&&this.type!==Un;for(let et=0,B=T.length;et<B;et++){let it=T[et],Z=it.shadow;if(Z===void 0){console.warn("THREE.WebGLShadowMap:",it,"has no shadow.");continue}if(Z.autoUpdate===!1&&Z.needsUpdate===!1)continue;s.copy(Z.mapSize);let gt=Z.getFrameExtents();if(s.multiply(gt),r.copy(Z.mapSize),(s.x>f||s.y>f)&&(s.x>f&&(r.x=Math.floor(f/gt.x),s.x=r.x*gt.x,Z.mapSize.x=r.x),s.y>f&&(r.y=Math.floor(f/gt.y),s.y=r.y*gt.y,Z.mapSize.y=r.y)),Z.map===null||V===!0||q===!0){let bt=this.type!==Un?{minFilter:Ee,magFilter:Ee}:{};Z.map!==null&&Z.map.dispose(),Z.map=new fe(s.x,s.y,bt),Z.map.texture.name=it.name+".shadowMap",Z.camera.updateProjectionMatrix()}n.setRenderTarget(Z.map),n.clear();let _t=Z.getViewportCount();for(let bt=0;bt<_t;bt++){let Gt=Z.getViewport(bt);o.set(r.x*Gt.x,r.y*Gt.y,r.x*Gt.z,r.y*Gt.w),F.viewport(o),Z.updateMatrices(it,bt),i=Z.getFrustum(),A(E,C,Z.camera,it,this.type)}Z.isPointLightShadow!==!0&&this.type===Un&&b(Z,C),Z.needsUpdate=!1}p=this.type,u.needsUpdate=!1,n.setRenderTarget(N,g,S)};function b(T,E){let C=t.update(_);h.defines.VSM_SAMPLES!==T.blurSamples&&(h.defines.VSM_SAMPLES=T.blurSamples,m.defines.VSM_SAMPLES=T.blurSamples,h.needsUpdate=!0,m.needsUpdate=!0),T.mapPass===null&&(T.mapPass=new fe(s.x,s.y)),h.uniforms.shadow_pass.value=T.map.texture,h.uniforms.resolution.value=T.mapSize,h.uniforms.radius.value=T.radius,n.setRenderTarget(T.mapPass),n.clear(),n.renderBufferDirect(E,null,C,h,_,null),m.uniforms.shadow_pass.value=T.mapPass.texture,m.uniforms.resolution.value=T.mapSize,m.uniforms.radius.value=T.radius,n.setRenderTarget(T.map),n.clear(),n.renderBufferDirect(E,null,C,m,_,null)}function y(T,E,C,N){let g=null,S=C.isPointLight===!0?T.customDistanceMaterial:T.customDepthMaterial;if(S!==void 0)g=S;else if(g=C.isPointLight===!0?c:a,n.localClippingEnabled&&E.clipShadows===!0&&Array.isArray(E.clippingPlanes)&&E.clippingPlanes.length!==0||E.displacementMap&&E.displacementScale!==0||E.alphaMap&&E.alphaTest>0||E.map&&E.alphaTest>0){let F=g.uuid,V=E.uuid,q=l[F];q===void 0&&(q={},l[F]=q);let et=q[V];et===void 0&&(et=g.clone(),q[V]=et,E.addEventListener("dispose",R)),g=et}if(g.visible=E.visible,g.wireframe=E.wireframe,N===Un?g.side=E.shadowSide!==null?E.shadowSide:E.side:g.side=E.shadowSide!==null?E.shadowSide:d[E.side],g.alphaMap=E.alphaMap,g.alphaTest=E.alphaTest,g.map=E.map,g.clipShadows=E.clipShadows,g.clippingPlanes=E.clippingPlanes,g.clipIntersection=E.clipIntersection,g.displacementMap=E.displacementMap,g.displacementScale=E.displacementScale,g.displacementBias=E.displacementBias,g.wireframeLinewidth=E.wireframeLinewidth,g.linewidth=E.linewidth,C.isPointLight===!0&&g.isMeshDistanceMaterial===!0){let F=n.properties.get(g);F.light=C}return g}function A(T,E,C,N,g){if(T.visible===!1)return;if(T.layers.test(E.layers)&&(T.isMesh||T.isLine||T.isPoints)&&(T.castShadow||T.receiveShadow&&g===Un)&&(!T.frustumCulled||i.intersectsObject(T))){T.modelViewMatrix.multiplyMatrices(C.matrixWorldInverse,T.matrixWorld);let V=t.update(T),q=T.material;if(Array.isArray(q)){let et=V.groups;for(let B=0,it=et.length;B<it;B++){let Z=et[B],gt=q[Z.materialIndex];if(gt&&gt.visible){let _t=y(T,gt,N,g);T.onBeforeShadow(n,T,E,C,V,_t,Z),n.renderBufferDirect(C,null,V,_t,T,Z),T.onAfterShadow(n,T,E,C,V,_t,Z)}}}else if(q.visible){let et=y(T,q,N,g);T.onBeforeShadow(n,T,E,C,V,et,null),n.renderBufferDirect(C,null,V,et,T,null),T.onAfterShadow(n,T,E,C,V,et,null)}}let F=T.children;for(let V=0,q=F.length;V<q;V++)A(F[V],E,C,N,g)}function R(T){T.target.removeEventListener("dispose",R);for(let C in l){let N=l[C],g=T.target.uuid;g in N&&(N[g].dispose(),delete N[g])}}}var gv={[_c]:yc,[Mc]:wc,[bc]:Ac,[xs]:Sc,[yc]:_c,[wc]:Mc,[Ac]:bc,[Sc]:xs};function xv(n){function t(){let L=!1,ft=new ce,Y=null,$=new ce(0,0,0,0);return{setMask:function(pt){Y!==pt&&!L&&(n.colorMask(pt,pt,pt,pt),Y=pt)},setLocked:function(pt){L=pt},setClear:function(pt,vt,Xt,ue,Fe){Fe===!0&&(pt*=ue,vt*=ue,Xt*=ue),ft.set(pt,vt,Xt,ue),$.equals(ft)===!1&&(n.clearColor(pt,vt,Xt,ue),$.copy(ft))},reset:function(){L=!1,Y=null,$.set(-1,0,0,0)}}}function e(){let L=!1,ft=!1,Y=null,$=null,pt=null;return{setReversed:function(vt){ft=vt},setTest:function(vt){vt?Mt(n.DEPTH_TEST):xt(n.DEPTH_TEST)},setMask:function(vt){Y!==vt&&!L&&(n.depthMask(vt),Y=vt)},setFunc:function(vt){if(ft&&(vt=gv[vt]),$!==vt){switch(vt){case _c:n.depthFunc(n.NEVER);break;case yc:n.depthFunc(n.ALWAYS);break;case Mc:n.depthFunc(n.LESS);break;case xs:n.depthFunc(n.LEQUAL);break;case bc:n.depthFunc(n.EQUAL);break;case Sc:n.depthFunc(n.GEQUAL);break;case wc:n.depthFunc(n.GREATER);break;case Ac:n.depthFunc(n.NOTEQUAL);break;default:n.depthFunc(n.LEQUAL)}$=vt}},setLocked:function(vt){L=vt},setClear:function(vt){pt!==vt&&(n.clearDepth(vt),pt=vt)},reset:function(){L=!1,Y=null,$=null,pt=null}}}function i(){let L=!1,ft=null,Y=null,$=null,pt=null,vt=null,Xt=null,ue=null,Fe=null;return{setTest:function(qt){L||(qt?Mt(n.STENCIL_TEST):xt(n.STENCIL_TEST))},setMask:function(qt){ft!==qt&&!L&&(n.stencilMask(qt),ft=qt)},setFunc:function(qt,Be,Cn){(Y!==qt||$!==Be||pt!==Cn)&&(n.stencilFunc(qt,Be,Cn),Y=qt,$=Be,pt=Cn)},setOp:function(qt,Be,Cn){(vt!==qt||Xt!==Be||ue!==Cn)&&(n.stencilOp(qt,Be,Cn),vt=qt,Xt=Be,ue=Cn)},setLocked:function(qt){L=qt},setClear:function(qt){Fe!==qt&&(n.clearStencil(qt),Fe=qt)},reset:function(){L=!1,ft=null,Y=null,$=null,pt=null,vt=null,Xt=null,ue=null,Fe=null}}}let s=new t,r=new e,o=new i,a=new WeakMap,c=new WeakMap,l={},f={},d=new WeakMap,h=[],m=null,x=!1,_=null,u=null,p=null,b=null,y=null,A=null,R=null,T=new Ot(0,0,0),E=0,C=!1,N=null,g=null,S=null,F=null,V=null,q=n.getParameter(n.MAX_COMBINED_TEXTURE_IMAGE_UNITS),et=!1,B=0,it=n.getParameter(n.VERSION);it.indexOf("WebGL")!==-1?(B=parseFloat(/^WebGL (\d)/.exec(it)[1]),et=B>=1):it.indexOf("OpenGL ES")!==-1&&(B=parseFloat(/^OpenGL ES (\d)/.exec(it)[1]),et=B>=2);let Z=null,gt={},_t=n.getParameter(n.SCISSOR_BOX),bt=n.getParameter(n.VIEWPORT),Gt=new ce().fromArray(_t),Wt=new ce().fromArray(bt);function J(L,ft,Y,$){let pt=new Uint8Array(4),vt=n.createTexture();n.bindTexture(L,vt),n.texParameteri(L,n.TEXTURE_MIN_FILTER,n.NEAREST),n.texParameteri(L,n.TEXTURE_MAG_FILTER,n.NEAREST);for(let Xt=0;Xt<Y;Xt++)L===n.TEXTURE_3D||L===n.TEXTURE_2D_ARRAY?n.texImage3D(ft,0,n.RGBA,1,1,$,0,n.RGBA,n.UNSIGNED_BYTE,pt):n.texImage2D(ft+Xt,0,n.RGBA,1,1,0,n.RGBA,n.UNSIGNED_BYTE,pt);return vt}let st={};st[n.TEXTURE_2D]=J(n.TEXTURE_2D,n.TEXTURE_2D,1),st[n.TEXTURE_CUBE_MAP]=J(n.TEXTURE_CUBE_MAP,n.TEXTURE_CUBE_MAP_POSITIVE_X,6),st[n.TEXTURE_2D_ARRAY]=J(n.TEXTURE_2D_ARRAY,n.TEXTURE_2D_ARRAY,1,1),st[n.TEXTURE_3D]=J(n.TEXTURE_3D,n.TEXTURE_3D,1,1),s.setClear(0,0,0,1),r.setClear(1),o.setClear(0),Mt(n.DEPTH_TEST),r.setFunc(xs),j(!1),ct(Fh),Mt(n.CULL_FACE),I(Sn);function Mt(L){l[L]!==!0&&(n.enable(L),l[L]=!0)}function xt(L){l[L]!==!1&&(n.disable(L),l[L]=!1)}function Nt(L,ft){return f[L]!==ft?(n.bindFramebuffer(L,ft),f[L]=ft,L===n.DRAW_FRAMEBUFFER&&(f[n.FRAMEBUFFER]=ft),L===n.FRAMEBUFFER&&(f[n.DRAW_FRAMEBUFFER]=ft),!0):!1}function It(L,ft){let Y=h,$=!1;if(L){Y=d.get(ft),Y===void 0&&(Y=[],d.set(ft,Y));let pt=L.textures;if(Y.length!==pt.length||Y[0]!==n.COLOR_ATTACHMENT0){for(let vt=0,Xt=pt.length;vt<Xt;vt++)Y[vt]=n.COLOR_ATTACHMENT0+vt;Y.length=pt.length,$=!0}}else Y[0]!==n.BACK&&(Y[0]=n.BACK,$=!0);$&&n.drawBuffers(Y)}function kt(L){return m!==L?(n.useProgram(L),m=L,!0):!1}let Zt={[bi]:n.FUNC_ADD,[If]:n.FUNC_SUBTRACT,[Lf]:n.FUNC_REVERSE_SUBTRACT};Zt[Df]=n.MIN,Zt[Uf]=n.MAX;let Vt={[Nf]:n.ZERO,[Of]:n.ONE,[Ff]:n.SRC_COLOR,[xc]:n.SRC_ALPHA,[Gf]:n.SRC_ALPHA_SATURATE,[Hf]:n.DST_COLOR,[zf]:n.DST_ALPHA,[Bf]:n.ONE_MINUS_SRC_COLOR,[vc]:n.ONE_MINUS_SRC_ALPHA,[Vf]:n.ONE_MINUS_DST_COLOR,[kf]:n.ONE_MINUS_DST_ALPHA,[Wf]:n.CONSTANT_COLOR,[Xf]:n.ONE_MINUS_CONSTANT_COLOR,[qf]:n.CONSTANT_ALPHA,[Yf]:n.ONE_MINUS_CONSTANT_ALPHA};function I(L,ft,Y,$,pt,vt,Xt,ue,Fe,qt){if(L===Sn){x===!0&&(xt(n.BLEND),x=!1);return}if(x===!1&&(Mt(n.BLEND),x=!0),L!==Pf){if(L!==_||qt!==C){if((u!==bi||y!==bi)&&(n.blendEquation(n.FUNC_ADD),u=bi,y=bi),qt)switch(L){case fs:n.blendFuncSeparate(n.ONE,n.ONE_MINUS_SRC_ALPHA,n.ONE,n.ONE_MINUS_SRC_ALPHA);break;case ni:n.blendFunc(n.ONE,n.ONE);break;case Bh:n.blendFuncSeparate(n.ZERO,n.ONE_MINUS_SRC_COLOR,n.ZERO,n.ONE);break;case zh:n.blendFuncSeparate(n.ZERO,n.SRC_COLOR,n.ZERO,n.SRC_ALPHA);break;default:console.error("THREE.WebGLState: Invalid blending: ",L);break}else switch(L){case fs:n.blendFuncSeparate(n.SRC_ALPHA,n.ONE_MINUS_SRC_ALPHA,n.ONE,n.ONE_MINUS_SRC_ALPHA);break;case ni:n.blendFunc(n.SRC_ALPHA,n.ONE);break;case Bh:n.blendFuncSeparate(n.ZERO,n.ONE_MINUS_SRC_COLOR,n.ZERO,n.ONE);break;case zh:n.blendFunc(n.ZERO,n.SRC_COLOR);break;default:console.error("THREE.WebGLState: Invalid blending: ",L);break}p=null,b=null,A=null,R=null,T.set(0,0,0),E=0,_=L,C=qt}return}pt=pt||ft,vt=vt||Y,Xt=Xt||$,(ft!==u||pt!==y)&&(n.blendEquationSeparate(Zt[ft],Zt[pt]),u=ft,y=pt),(Y!==p||$!==b||vt!==A||Xt!==R)&&(n.blendFuncSeparate(Vt[Y],Vt[$],Vt[vt],Vt[Xt]),p=Y,b=$,A=vt,R=Xt),(ue.equals(T)===!1||Fe!==E)&&(n.blendColor(ue.r,ue.g,ue.b,Fe),T.copy(ue),E=Fe),_=L,C=!1}function me(L,ft){L.side===Nn?xt(n.CULL_FACE):Mt(n.CULL_FACE);let Y=L.side===ze;ft&&(Y=!Y),j(Y),L.blending===fs&&L.transparent===!1?I(Sn):I(L.blending,L.blendEquation,L.blendSrc,L.blendDst,L.blendEquationAlpha,L.blendSrcAlpha,L.blendDstAlpha,L.blendColor,L.blendAlpha,L.premultipliedAlpha),r.setFunc(L.depthFunc),r.setTest(L.depthTest),r.setMask(L.depthWrite),s.setMask(L.colorWrite);let $=L.stencilWrite;o.setTest($),$&&(o.setMask(L.stencilWriteMask),o.setFunc(L.stencilFunc,L.stencilRef,L.stencilFuncMask),o.setOp(L.stencilFail,L.stencilZFail,L.stencilZPass)),Rt(L.polygonOffset,L.polygonOffsetFactor,L.polygonOffsetUnits),L.alphaToCoverage===!0?Mt(n.SAMPLE_ALPHA_TO_COVERAGE):xt(n.SAMPLE_ALPHA_TO_COVERAGE)}function j(L){N!==L&&(L?n.frontFace(n.CW):n.frontFace(n.CCW),N=L)}function ct(L){L!==Tf?(Mt(n.CULL_FACE),L!==g&&(L===Fh?n.cullFace(n.BACK):L===Cf?n.cullFace(n.FRONT):n.cullFace(n.FRONT_AND_BACK))):xt(n.CULL_FACE),g=L}function ut(L){L!==S&&(et&&n.lineWidth(L),S=L)}function Rt(L,ft,Y){L?(Mt(n.POLYGON_OFFSET_FILL),(F!==ft||V!==Y)&&(n.polygonOffset(ft,Y),F=ft,V=Y)):xt(n.POLYGON_OFFSET_FILL)}function z(L){L?Mt(n.SCISSOR_TEST):xt(n.SCISSOR_TEST)}function M(L){L===void 0&&(L=n.TEXTURE0+q-1),Z!==L&&(n.activeTexture(L),Z=L)}function v(L,ft,Y){Y===void 0&&(Z===null?Y=n.TEXTURE0+q-1:Y=Z);let $=gt[Y];$===void 0&&($={type:void 0,texture:void 0},gt[Y]=$),($.type!==L||$.texture!==ft)&&(Z!==Y&&(n.activeTexture(Y),Z=Y),n.bindTexture(L,ft||st[L]),$.type=L,$.texture=ft)}function P(){let L=gt[Z];L!==void 0&&L.type!==void 0&&(n.bindTexture(L.type,null),L.type=void 0,L.texture=void 0)}function H(){try{n.compressedTexImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function K(){try{n.compressedTexImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function k(){try{n.texSubImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function nt(){try{n.texSubImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function Q(){try{n.compressedTexSubImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function X(){try{n.compressedTexSubImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function ht(){try{n.texStorage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function tt(){try{n.texStorage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function lt(){try{n.texImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function At(){try{n.texImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function rt(L){Gt.equals(L)===!1&&(n.scissor(L.x,L.y,L.z,L.w),Gt.copy(L))}function ot(L){Wt.equals(L)===!1&&(n.viewport(L.x,L.y,L.z,L.w),Wt.copy(L))}function Lt(L,ft){let Y=c.get(ft);Y===void 0&&(Y=new WeakMap,c.set(ft,Y));let $=Y.get(L);$===void 0&&($=n.getUniformBlockIndex(ft,L.name),Y.set(L,$))}function wt(L,ft){let $=c.get(ft).get(L);a.get(ft)!==$&&(n.uniformBlockBinding(ft,$,L.__bindingPointIndex),a.set(ft,$))}function Ft(){n.disable(n.BLEND),n.disable(n.CULL_FACE),n.disable(n.DEPTH_TEST),n.disable(n.POLYGON_OFFSET_FILL),n.disable(n.SCISSOR_TEST),n.disable(n.STENCIL_TEST),n.disable(n.SAMPLE_ALPHA_TO_COVERAGE),n.blendEquation(n.FUNC_ADD),n.blendFunc(n.ONE,n.ZERO),n.blendFuncSeparate(n.ONE,n.ZERO,n.ONE,n.ZERO),n.blendColor(0,0,0,0),n.colorMask(!0,!0,!0,!0),n.clearColor(0,0,0,0),n.depthMask(!0),n.depthFunc(n.LESS),n.clearDepth(1),n.stencilMask(4294967295),n.stencilFunc(n.ALWAYS,0,4294967295),n.stencilOp(n.KEEP,n.KEEP,n.KEEP),n.clearStencil(0),n.cullFace(n.BACK),n.frontFace(n.CCW),n.polygonOffset(0,0),n.activeTexture(n.TEXTURE0),n.bindFramebuffer(n.FRAMEBUFFER,null),n.bindFramebuffer(n.DRAW_FRAMEBUFFER,null),n.bindFramebuffer(n.READ_FRAMEBUFFER,null),n.useProgram(null),n.lineWidth(1),n.scissor(0,0,n.canvas.width,n.canvas.height),n.viewport(0,0,n.canvas.width,n.canvas.height),l={},Z=null,gt={},f={},d=new WeakMap,h=[],m=null,x=!1,_=null,u=null,p=null,b=null,y=null,A=null,R=null,T=new Ot(0,0,0),E=0,C=!1,N=null,g=null,S=null,F=null,V=null,Gt.set(0,0,n.canvas.width,n.canvas.height),Wt.set(0,0,n.canvas.width,n.canvas.height),s.reset(),r.reset(),o.reset()}return{buffers:{color:s,depth:r,stencil:o},enable:Mt,disable:xt,bindFramebuffer:Nt,drawBuffers:It,useProgram:kt,setBlending:I,setMaterial:me,setFlipSided:j,setCullFace:ct,setLineWidth:ut,setPolygonOffset:Rt,setScissorTest:z,activeTexture:M,bindTexture:v,unbindTexture:P,compressedTexImage2D:H,compressedTexImage3D:K,texImage2D:lt,texImage3D:At,updateUBOMapping:Lt,uniformBlockBinding:wt,texStorage2D:ht,texStorage3D:tt,texSubImage2D:k,texSubImage3D:nt,compressedTexSubImage2D:Q,compressedTexSubImage3D:X,scissor:rt,viewport:ot,reset:Ft}}function Lu(n,t,e,i){let s=vv(i);switch(e){case qu:return n*t;case Zu:return n*t;case ju:return n*t*2;case Fl:return n*t/s.components*s.byteLength;case Bl:return n*t/s.components*s.byteLength;case Ku:return n*t*2/s.components*s.byteLength;case zl:return n*t*2/s.components*s.byteLength;case Yu:return n*t*3/s.components*s.byteLength;case fn:return n*t*4/s.components*s.byteLength;case kl:return n*t*4/s.components*s.byteLength;case ao:case co:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*8;case lo:case ho:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case Ic:case Dc:return Math.max(n,16)*Math.max(t,8)/4;case Pc:case Lc:return Math.max(n,8)*Math.max(t,8)/2;case Uc:case Nc:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*8;case Oc:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case Fc:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case Bc:return Math.floor((n+4)/5)*Math.floor((t+3)/4)*16;case zc:return Math.floor((n+4)/5)*Math.floor((t+4)/5)*16;case kc:return Math.floor((n+5)/6)*Math.floor((t+4)/5)*16;case Hc:return Math.floor((n+5)/6)*Math.floor((t+5)/6)*16;case Vc:return Math.floor((n+7)/8)*Math.floor((t+4)/5)*16;case Gc:return Math.floor((n+7)/8)*Math.floor((t+5)/6)*16;case Wc:return Math.floor((n+7)/8)*Math.floor((t+7)/8)*16;case Xc:return Math.floor((n+9)/10)*Math.floor((t+4)/5)*16;case qc:return Math.floor((n+9)/10)*Math.floor((t+5)/6)*16;case Yc:return Math.floor((n+9)/10)*Math.floor((t+7)/8)*16;case Zc:return Math.floor((n+9)/10)*Math.floor((t+9)/10)*16;case jc:return Math.floor((n+11)/12)*Math.floor((t+9)/10)*16;case Kc:return Math.floor((n+11)/12)*Math.floor((t+11)/12)*16;case uo:case Jc:case Qc:return Math.ceil(n/4)*Math.ceil(t/4)*16;case Ju:case $c:return Math.ceil(n/4)*Math.ceil(t/4)*8;case tl:case el:return Math.ceil(n/4)*Math.ceil(t/4)*16}throw new Error(`Unable to determine texture byte length for ${e} format.`)}function vv(n){switch(n){case Fn:case Gu:return{byteLength:1,components:1};case ar:case Wu:case Ve:return{byteLength:2,components:1};case Nl:case Ol:return{byteLength:2,components:4};case Ti:case Ul:case bn:return{byteLength:4,components:1};case Xu:return{byteLength:4,components:3}}throw new Error(`Unknown texture type ${n}.`)}function _v(n,t,e,i,s,r,o){let a=t.has("WEBGL_multisampled_render_to_texture")?t.get("WEBGL_multisampled_render_to_texture"):null,c=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),l=new yt,f=new WeakMap,d,h=new WeakMap,m=!1;try{m=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function x(M,v){return m?new OffscreenCanvas(M,v):_o("canvas")}function _(M,v,P){let H=1,K=z(M);if((K.width>P||K.height>P)&&(H=P/Math.max(K.width,K.height)),H<1)if(typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&M instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&M instanceof ImageBitmap||typeof VideoFrame<"u"&&M instanceof VideoFrame){let k=Math.floor(H*K.width),nt=Math.floor(H*K.height);d===void 0&&(d=x(k,nt));let Q=v?x(k,nt):d;return Q.width=k,Q.height=nt,Q.getContext("2d").drawImage(M,0,0,k,nt),console.warn("THREE.WebGLRenderer: Texture has been resized from ("+K.width+"x"+K.height+") to ("+k+"x"+nt+")."),Q}else return"data"in M&&console.warn("THREE.WebGLRenderer: Image in DataTexture is too big ("+K.width+"x"+K.height+")."),M;return M}function u(M){return M.generateMipmaps&&M.minFilter!==Ee&&M.minFilter!==je}function p(M){n.generateMipmap(M)}function b(M,v,P,H,K=!1){if(M!==null){if(n[M]!==void 0)return n[M];console.warn("THREE.WebGLRenderer: Attempt to use non-existing WebGL internal format '"+M+"'")}let k=v;if(v===n.RED&&(P===n.FLOAT&&(k=n.R32F),P===n.HALF_FLOAT&&(k=n.R16F),P===n.UNSIGNED_BYTE&&(k=n.R8)),v===n.RED_INTEGER&&(P===n.UNSIGNED_BYTE&&(k=n.R8UI),P===n.UNSIGNED_SHORT&&(k=n.R16UI),P===n.UNSIGNED_INT&&(k=n.R32UI),P===n.BYTE&&(k=n.R8I),P===n.SHORT&&(k=n.R16I),P===n.INT&&(k=n.R32I)),v===n.RG&&(P===n.FLOAT&&(k=n.RG32F),P===n.HALF_FLOAT&&(k=n.RG16F),P===n.UNSIGNED_BYTE&&(k=n.RG8)),v===n.RG_INTEGER&&(P===n.UNSIGNED_BYTE&&(k=n.RG8UI),P===n.UNSIGNED_SHORT&&(k=n.RG16UI),P===n.UNSIGNED_INT&&(k=n.RG32UI),P===n.BYTE&&(k=n.RG8I),P===n.SHORT&&(k=n.RG16I),P===n.INT&&(k=n.RG32I)),v===n.RGB_INTEGER&&(P===n.UNSIGNED_BYTE&&(k=n.RGB8UI),P===n.UNSIGNED_SHORT&&(k=n.RGB16UI),P===n.UNSIGNED_INT&&(k=n.RGB32UI),P===n.BYTE&&(k=n.RGB8I),P===n.SHORT&&(k=n.RGB16I),P===n.INT&&(k=n.RGB32I)),v===n.RGBA_INTEGER&&(P===n.UNSIGNED_BYTE&&(k=n.RGBA8UI),P===n.UNSIGNED_SHORT&&(k=n.RGBA16UI),P===n.UNSIGNED_INT&&(k=n.RGBA32UI),P===n.BYTE&&(k=n.RGBA8I),P===n.SHORT&&(k=n.RGBA16I),P===n.INT&&(k=n.RGBA32I)),v===n.RGB&&P===n.UNSIGNED_INT_5_9_9_9_REV&&(k=n.RGB9_E5),v===n.RGBA){let nt=K?mo:Jt.getTransfer(H);P===n.FLOAT&&(k=n.RGBA32F),P===n.HALF_FLOAT&&(k=n.RGBA16F),P===n.UNSIGNED_BYTE&&(k=nt===ie?n.SRGB8_ALPHA8:n.RGBA8),P===n.UNSIGNED_SHORT_4_4_4_4&&(k=n.RGBA4),P===n.UNSIGNED_SHORT_5_5_5_1&&(k=n.RGB5_A1)}return(k===n.R16F||k===n.R32F||k===n.RG16F||k===n.RG32F||k===n.RGBA16F||k===n.RGBA32F)&&t.get("EXT_color_buffer_float"),k}function y(M,v){let P;return M?v===null||v===Ti||v===ys?P=n.DEPTH24_STENCIL8:v===bn?P=n.DEPTH32F_STENCIL8:v===ar&&(P=n.DEPTH24_STENCIL8,console.warn("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):v===null||v===Ti||v===ys?P=n.DEPTH_COMPONENT24:v===bn?P=n.DEPTH_COMPONENT32F:v===ar&&(P=n.DEPTH_COMPONENT16),P}function A(M,v){return u(M)===!0||M.isFramebufferTexture&&M.minFilter!==Ee&&M.minFilter!==je?Math.log2(Math.max(v.width,v.height))+1:M.mipmaps!==void 0&&M.mipmaps.length>0?M.mipmaps.length:M.isCompressedTexture&&Array.isArray(M.image)?v.mipmaps.length:1}function R(M){let v=M.target;v.removeEventListener("dispose",R),E(v),v.isVideoTexture&&f.delete(v)}function T(M){let v=M.target;v.removeEventListener("dispose",T),N(v)}function E(M){let v=i.get(M);if(v.__webglInit===void 0)return;let P=M.source,H=h.get(P);if(H){let K=H[v.__cacheKey];K.usedTimes--,K.usedTimes===0&&C(M),Object.keys(H).length===0&&h.delete(P)}i.remove(M)}function C(M){let v=i.get(M);n.deleteTexture(v.__webglTexture);let P=M.source,H=h.get(P);delete H[v.__cacheKey],o.memory.textures--}function N(M){let v=i.get(M);if(M.depthTexture&&M.depthTexture.dispose(),M.isWebGLCubeRenderTarget)for(let H=0;H<6;H++){if(Array.isArray(v.__webglFramebuffer[H]))for(let K=0;K<v.__webglFramebuffer[H].length;K++)n.deleteFramebuffer(v.__webglFramebuffer[H][K]);else n.deleteFramebuffer(v.__webglFramebuffer[H]);v.__webglDepthbuffer&&n.deleteRenderbuffer(v.__webglDepthbuffer[H])}else{if(Array.isArray(v.__webglFramebuffer))for(let H=0;H<v.__webglFramebuffer.length;H++)n.deleteFramebuffer(v.__webglFramebuffer[H]);else n.deleteFramebuffer(v.__webglFramebuffer);if(v.__webglDepthbuffer&&n.deleteRenderbuffer(v.__webglDepthbuffer),v.__webglMultisampledFramebuffer&&n.deleteFramebuffer(v.__webglMultisampledFramebuffer),v.__webglColorRenderbuffer)for(let H=0;H<v.__webglColorRenderbuffer.length;H++)v.__webglColorRenderbuffer[H]&&n.deleteRenderbuffer(v.__webglColorRenderbuffer[H]);v.__webglDepthRenderbuffer&&n.deleteRenderbuffer(v.__webglDepthRenderbuffer)}let P=M.textures;for(let H=0,K=P.length;H<K;H++){let k=i.get(P[H]);k.__webglTexture&&(n.deleteTexture(k.__webglTexture),o.memory.textures--),i.remove(P[H])}i.remove(M)}let g=0;function S(){g=0}function F(){let M=g;return M>=s.maxTextures&&console.warn("THREE.WebGLTextures: Trying to use "+M+" texture units while this GPU supports only "+s.maxTextures),g+=1,M}function V(M){let v=[];return v.push(M.wrapS),v.push(M.wrapT),v.push(M.wrapR||0),v.push(M.magFilter),v.push(M.minFilter),v.push(M.anisotropy),v.push(M.internalFormat),v.push(M.format),v.push(M.type),v.push(M.generateMipmaps),v.push(M.premultiplyAlpha),v.push(M.flipY),v.push(M.unpackAlignment),v.push(M.colorSpace),v.join()}function q(M,v){let P=i.get(M);if(M.isVideoTexture&&ut(M),M.isRenderTargetTexture===!1&&M.version>0&&P.__version!==M.version){let H=M.image;if(H===null)console.warn("THREE.WebGLRenderer: Texture marked for update but no image data found.");else if(H.complete===!1)console.warn("THREE.WebGLRenderer: Texture marked for update but image is incomplete");else{Wt(P,M,v);return}}e.bindTexture(n.TEXTURE_2D,P.__webglTexture,n.TEXTURE0+v)}function et(M,v){let P=i.get(M);if(M.version>0&&P.__version!==M.version){Wt(P,M,v);return}e.bindTexture(n.TEXTURE_2D_ARRAY,P.__webglTexture,n.TEXTURE0+v)}function B(M,v){let P=i.get(M);if(M.version>0&&P.__version!==M.version){Wt(P,M,v);return}e.bindTexture(n.TEXTURE_3D,P.__webglTexture,n.TEXTURE0+v)}function it(M,v){let P=i.get(M);if(M.version>0&&P.__version!==M.version){J(P,M,v);return}e.bindTexture(n.TEXTURE_CUBE_MAP,P.__webglTexture,n.TEXTURE0+v)}let Z={[Cc]:n.REPEAT,[Ai]:n.CLAMP_TO_EDGE,[Rc]:n.MIRRORED_REPEAT},gt={[Ee]:n.NEAREST,[ip]:n.NEAREST_MIPMAP_NEAREST,[Br]:n.NEAREST_MIPMAP_LINEAR,[je]:n.LINEAR,[Ha]:n.LINEAR_MIPMAP_NEAREST,[Ei]:n.LINEAR_MIPMAP_LINEAR},_t={[ap]:n.NEVER,[fp]:n.ALWAYS,[cp]:n.LESS,[$u]:n.LEQUAL,[lp]:n.EQUAL,[dp]:n.GEQUAL,[hp]:n.GREATER,[up]:n.NOTEQUAL};function bt(M,v){if(v.type===bn&&t.has("OES_texture_float_linear")===!1&&(v.magFilter===je||v.magFilter===Ha||v.magFilter===Br||v.magFilter===Ei||v.minFilter===je||v.minFilter===Ha||v.minFilter===Br||v.minFilter===Ei)&&console.warn("THREE.WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),n.texParameteri(M,n.TEXTURE_WRAP_S,Z[v.wrapS]),n.texParameteri(M,n.TEXTURE_WRAP_T,Z[v.wrapT]),(M===n.TEXTURE_3D||M===n.TEXTURE_2D_ARRAY)&&n.texParameteri(M,n.TEXTURE_WRAP_R,Z[v.wrapR]),n.texParameteri(M,n.TEXTURE_MAG_FILTER,gt[v.magFilter]),n.texParameteri(M,n.TEXTURE_MIN_FILTER,gt[v.minFilter]),v.compareFunction&&(n.texParameteri(M,n.TEXTURE_COMPARE_MODE,n.COMPARE_REF_TO_TEXTURE),n.texParameteri(M,n.TEXTURE_COMPARE_FUNC,_t[v.compareFunction])),t.has("EXT_texture_filter_anisotropic")===!0){if(v.magFilter===Ee||v.minFilter!==Br&&v.minFilter!==Ei||v.type===bn&&t.has("OES_texture_float_linear")===!1)return;if(v.anisotropy>1||i.get(v).__currentAnisotropy){let P=t.get("EXT_texture_filter_anisotropic");n.texParameterf(M,P.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(v.anisotropy,s.getMaxAnisotropy())),i.get(v).__currentAnisotropy=v.anisotropy}}}function Gt(M,v){let P=!1;M.__webglInit===void 0&&(M.__webglInit=!0,v.addEventListener("dispose",R));let H=v.source,K=h.get(H);K===void 0&&(K={},h.set(H,K));let k=V(v);if(k!==M.__cacheKey){K[k]===void 0&&(K[k]={texture:n.createTexture(),usedTimes:0},o.memory.textures++,P=!0),K[k].usedTimes++;let nt=K[M.__cacheKey];nt!==void 0&&(K[M.__cacheKey].usedTimes--,nt.usedTimes===0&&C(v)),M.__cacheKey=k,M.__webglTexture=K[k].texture}return P}function Wt(M,v,P){let H=n.TEXTURE_2D;(v.isDataArrayTexture||v.isCompressedArrayTexture)&&(H=n.TEXTURE_2D_ARRAY),v.isData3DTexture&&(H=n.TEXTURE_3D);let K=Gt(M,v),k=v.source;e.bindTexture(H,M.__webglTexture,n.TEXTURE0+P);let nt=i.get(k);if(k.version!==nt.__version||K===!0){e.activeTexture(n.TEXTURE0+P);let Q=Jt.getPrimaries(Jt.workingColorSpace),X=v.colorSpace===Qn?null:Jt.getPrimaries(v.colorSpace),ht=v.colorSpace===Qn||Q===X?n.NONE:n.BROWSER_DEFAULT_WEBGL;n.pixelStorei(n.UNPACK_FLIP_Y_WEBGL,v.flipY),n.pixelStorei(n.UNPACK_PREMULTIPLY_ALPHA_WEBGL,v.premultiplyAlpha),n.pixelStorei(n.UNPACK_ALIGNMENT,v.unpackAlignment),n.pixelStorei(n.UNPACK_COLORSPACE_CONVERSION_WEBGL,ht);let tt=_(v.image,!1,s.maxTextureSize);tt=Rt(v,tt);let lt=r.convert(v.format,v.colorSpace),At=r.convert(v.type),rt=b(v.internalFormat,lt,At,v.colorSpace,v.isVideoTexture);bt(H,v);let ot,Lt=v.mipmaps,wt=v.isVideoTexture!==!0,Ft=nt.__version===void 0||K===!0,L=k.dataReady,ft=A(v,tt);if(v.isDepthTexture)rt=y(v.format===Ms,v.type),Ft&&(wt?e.texStorage2D(n.TEXTURE_2D,1,rt,tt.width,tt.height):e.texImage2D(n.TEXTURE_2D,0,rt,tt.width,tt.height,0,lt,At,null));else if(v.isDataTexture)if(Lt.length>0){wt&&Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,Lt[0].width,Lt[0].height);for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,lt,At,ot.data):e.texImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,lt,At,ot.data);v.generateMipmaps=!1}else wt?(Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,tt.width,tt.height),L&&e.texSubImage2D(n.TEXTURE_2D,0,0,0,tt.width,tt.height,lt,At,tt.data)):e.texImage2D(n.TEXTURE_2D,0,rt,tt.width,tt.height,0,lt,At,tt.data);else if(v.isCompressedTexture)if(v.isCompressedArrayTexture){wt&&Ft&&e.texStorage3D(n.TEXTURE_2D_ARRAY,ft,rt,Lt[0].width,Lt[0].height,tt.depth);for(let Y=0,$=Lt.length;Y<$;Y++)if(ot=Lt[Y],v.format!==fn)if(lt!==null)if(wt){if(L)if(v.layerUpdates.size>0){let pt=Lu(ot.width,ot.height,v.format,v.type);for(let vt of v.layerUpdates){let Xt=ot.data.subarray(vt*pt/ot.data.BYTES_PER_ELEMENT,(vt+1)*pt/ot.data.BYTES_PER_ELEMENT);e.compressedTexSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,vt,ot.width,ot.height,1,lt,Xt,0,0)}v.clearLayerUpdates()}else e.compressedTexSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,0,ot.width,ot.height,tt.depth,lt,ot.data,0,0)}else e.compressedTexImage3D(n.TEXTURE_2D_ARRAY,Y,rt,ot.width,ot.height,tt.depth,0,ot.data,0,0);else console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else wt?L&&e.texSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,0,ot.width,ot.height,tt.depth,lt,At,ot.data):e.texImage3D(n.TEXTURE_2D_ARRAY,Y,rt,ot.width,ot.height,tt.depth,0,lt,At,ot.data)}else{wt&&Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,Lt[0].width,Lt[0].height);for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],v.format!==fn?lt!==null?wt?L&&e.compressedTexSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,lt,ot.data):e.compressedTexImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,ot.data):console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,lt,At,ot.data):e.texImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,lt,At,ot.data)}else if(v.isDataArrayTexture)if(wt){if(Ft&&e.texStorage3D(n.TEXTURE_2D_ARRAY,ft,rt,tt.width,tt.height,tt.depth),L)if(v.layerUpdates.size>0){let Y=Lu(tt.width,tt.height,v.format,v.type);for(let $ of v.layerUpdates){let pt=tt.data.subarray($*Y/tt.data.BYTES_PER_ELEMENT,($+1)*Y/tt.data.BYTES_PER_ELEMENT);e.texSubImage3D(n.TEXTURE_2D_ARRAY,0,0,0,$,tt.width,tt.height,1,lt,At,pt)}v.clearLayerUpdates()}else e.texSubImage3D(n.TEXTURE_2D_ARRAY,0,0,0,0,tt.width,tt.height,tt.depth,lt,At,tt.data)}else e.texImage3D(n.TEXTURE_2D_ARRAY,0,rt,tt.width,tt.height,tt.depth,0,lt,At,tt.data);else if(v.isData3DTexture)wt?(Ft&&e.texStorage3D(n.TEXTURE_3D,ft,rt,tt.width,tt.height,tt.depth),L&&e.texSubImage3D(n.TEXTURE_3D,0,0,0,0,tt.width,tt.height,tt.depth,lt,At,tt.data)):e.texImage3D(n.TEXTURE_3D,0,rt,tt.width,tt.height,tt.depth,0,lt,At,tt.data);else if(v.isFramebufferTexture){if(Ft)if(wt)e.texStorage2D(n.TEXTURE_2D,ft,rt,tt.width,tt.height);else{let Y=tt.width,$=tt.height;for(let pt=0;pt<ft;pt++)e.texImage2D(n.TEXTURE_2D,pt,rt,Y,$,0,lt,At,null),Y>>=1,$>>=1}}else if(Lt.length>0){if(wt&&Ft){let Y=z(Lt[0]);e.texStorage2D(n.TEXTURE_2D,ft,rt,Y.width,Y.height)}for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,lt,At,ot):e.texImage2D(n.TEXTURE_2D,Y,rt,lt,At,ot);v.generateMipmaps=!1}else if(wt){if(Ft){let Y=z(tt);e.texStorage2D(n.TEXTURE_2D,ft,rt,Y.width,Y.height)}L&&e.texSubImage2D(n.TEXTURE_2D,0,0,0,lt,At,tt)}else e.texImage2D(n.TEXTURE_2D,0,rt,lt,At,tt);u(v)&&p(H),nt.__version=k.version,v.onUpdate&&v.onUpdate(v)}M.__version=v.version}function J(M,v,P){if(v.image.length!==6)return;let H=Gt(M,v),K=v.source;e.bindTexture(n.TEXTURE_CUBE_MAP,M.__webglTexture,n.TEXTURE0+P);let k=i.get(K);if(K.version!==k.__version||H===!0){e.activeTexture(n.TEXTURE0+P);let nt=Jt.getPrimaries(Jt.workingColorSpace),Q=v.colorSpace===Qn?null:Jt.getPrimaries(v.colorSpace),X=v.colorSpace===Qn||nt===Q?n.NONE:n.BROWSER_DEFAULT_WEBGL;n.pixelStorei(n.UNPACK_FLIP_Y_WEBGL,v.flipY),n.pixelStorei(n.UNPACK_PREMULTIPLY_ALPHA_WEBGL,v.premultiplyAlpha),n.pixelStorei(n.UNPACK_ALIGNMENT,v.unpackAlignment),n.pixelStorei(n.UNPACK_COLORSPACE_CONVERSION_WEBGL,X);let ht=v.isCompressedTexture||v.image[0].isCompressedTexture,tt=v.image[0]&&v.image[0].isDataTexture,lt=[];for(let $=0;$<6;$++)!ht&&!tt?lt[$]=_(v.image[$],!0,s.maxCubemapSize):lt[$]=tt?v.image[$].image:v.image[$],lt[$]=Rt(v,lt[$]);let At=lt[0],rt=r.convert(v.format,v.colorSpace),ot=r.convert(v.type),Lt=b(v.internalFormat,rt,ot,v.colorSpace),wt=v.isVideoTexture!==!0,Ft=k.__version===void 0||H===!0,L=K.dataReady,ft=A(v,At);bt(n.TEXTURE_CUBE_MAP,v);let Y;if(ht){wt&&Ft&&e.texStorage2D(n.TEXTURE_CUBE_MAP,ft,Lt,At.width,At.height);for(let $=0;$<6;$++){Y=lt[$].mipmaps;for(let pt=0;pt<Y.length;pt++){let vt=Y[pt];v.format!==fn?rt!==null?wt?L&&e.compressedTexSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,0,0,vt.width,vt.height,rt,vt.data):e.compressedTexImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,Lt,vt.width,vt.height,0,vt.data):console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,0,0,vt.width,vt.height,rt,ot,vt.data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,Lt,vt.width,vt.height,0,rt,ot,vt.data)}}}else{if(Y=v.mipmaps,wt&&Ft){Y.length>0&&ft++;let $=z(lt[0]);e.texStorage2D(n.TEXTURE_CUBE_MAP,ft,Lt,$.width,$.height)}for(let $=0;$<6;$++)if(tt){wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,0,0,lt[$].width,lt[$].height,rt,ot,lt[$].data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,Lt,lt[$].width,lt[$].height,0,rt,ot,lt[$].data);for(let pt=0;pt<Y.length;pt++){let Xt=Y[pt].image[$].image;wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,0,0,Xt.width,Xt.height,rt,ot,Xt.data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,Lt,Xt.width,Xt.height,0,rt,ot,Xt.data)}}else{wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,0,0,rt,ot,lt[$]):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,Lt,rt,ot,lt[$]);for(let pt=0;pt<Y.length;pt++){let vt=Y[pt];wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,0,0,rt,ot,vt.image[$]):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,Lt,rt,ot,vt.image[$])}}}u(v)&&p(n.TEXTURE_CUBE_MAP),k.__version=K.version,v.onUpdate&&v.onUpdate(v)}M.__version=v.version}function st(M,v,P,H,K,k){let nt=r.convert(P.format,P.colorSpace),Q=r.convert(P.type),X=b(P.internalFormat,nt,Q,P.colorSpace);if(!i.get(v).__hasExternalTextures){let tt=Math.max(1,v.width>>k),lt=Math.max(1,v.height>>k);K===n.TEXTURE_3D||K===n.TEXTURE_2D_ARRAY?e.texImage3D(K,k,X,tt,lt,v.depth,0,nt,Q,null):e.texImage2D(K,k,X,tt,lt,0,nt,Q,null)}e.bindFramebuffer(n.FRAMEBUFFER,M),ct(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,H,K,i.get(P).__webglTexture,0,j(v)):(K===n.TEXTURE_2D||K>=n.TEXTURE_CUBE_MAP_POSITIVE_X&&K<=n.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&n.framebufferTexture2D(n.FRAMEBUFFER,H,K,i.get(P).__webglTexture,k),e.bindFramebuffer(n.FRAMEBUFFER,null)}function Mt(M,v,P){if(n.bindRenderbuffer(n.RENDERBUFFER,M),v.depthBuffer){let H=v.depthTexture,K=H&&H.isDepthTexture?H.type:null,k=y(v.stencilBuffer,K),nt=v.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,Q=j(v);ct(v)?a.renderbufferStorageMultisampleEXT(n.RENDERBUFFER,Q,k,v.width,v.height):P?n.renderbufferStorageMultisample(n.RENDERBUFFER,Q,k,v.width,v.height):n.renderbufferStorage(n.RENDERBUFFER,k,v.width,v.height),n.framebufferRenderbuffer(n.FRAMEBUFFER,nt,n.RENDERBUFFER,M)}else{let H=v.textures;for(let K=0;K<H.length;K++){let k=H[K],nt=r.convert(k.format,k.colorSpace),Q=r.convert(k.type),X=b(k.internalFormat,nt,Q,k.colorSpace),ht=j(v);P&&ct(v)===!1?n.renderbufferStorageMultisample(n.RENDERBUFFER,ht,X,v.width,v.height):ct(v)?a.renderbufferStorageMultisampleEXT(n.RENDERBUFFER,ht,X,v.width,v.height):n.renderbufferStorage(n.RENDERBUFFER,X,v.width,v.height)}}n.bindRenderbuffer(n.RENDERBUFFER,null)}function xt(M,v){if(v&&v.isWebGLCubeRenderTarget)throw new Error("Depth Texture with cube render targets is not supported");if(e.bindFramebuffer(n.FRAMEBUFFER,M),!(v.depthTexture&&v.depthTexture.isDepthTexture))throw new Error("renderTarget.depthTexture must be an instance of THREE.DepthTexture");(!i.get(v.depthTexture).__webglTexture||v.depthTexture.image.width!==v.width||v.depthTexture.image.height!==v.height)&&(v.depthTexture.image.width=v.width,v.depthTexture.image.height=v.height,v.depthTexture.needsUpdate=!0),q(v.depthTexture,0);let H=i.get(v.depthTexture).__webglTexture,K=j(v);if(v.depthTexture.format===ps)ct(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,n.DEPTH_ATTACHMENT,n.TEXTURE_2D,H,0,K):n.framebufferTexture2D(n.FRAMEBUFFER,n.DEPTH_ATTACHMENT,n.TEXTURE_2D,H,0);else if(v.depthTexture.format===Ms)ct(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,n.DEPTH_STENCIL_ATTACHMENT,n.TEXTURE_2D,H,0,K):n.framebufferTexture2D(n.FRAMEBUFFER,n.DEPTH_STENCIL_ATTACHMENT,n.TEXTURE_2D,H,0);else throw new Error("Unknown depthTexture format")}function Nt(M){let v=i.get(M),P=M.isWebGLCubeRenderTarget===!0;if(v.__boundDepthTexture!==M.depthTexture){let H=M.depthTexture;if(v.__depthDisposeCallback&&v.__depthDisposeCallback(),H){let K=()=>{delete v.__boundDepthTexture,delete v.__depthDisposeCallback,H.removeEventListener("dispose",K)};H.addEventListener("dispose",K),v.__depthDisposeCallback=K}v.__boundDepthTexture=H}if(M.depthTexture&&!v.__autoAllocateDepthBuffer){if(P)throw new Error("target.depthTexture not supported in Cube render targets");xt(v.__webglFramebuffer,M)}else if(P){v.__webglDepthbuffer=[];for(let H=0;H<6;H++)if(e.bindFramebuffer(n.FRAMEBUFFER,v.__webglFramebuffer[H]),v.__webglDepthbuffer[H]===void 0)v.__webglDepthbuffer[H]=n.createRenderbuffer(),Mt(v.__webglDepthbuffer[H],M,!1);else{let K=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,k=v.__webglDepthbuffer[H];n.bindRenderbuffer(n.RENDERBUFFER,k),n.framebufferRenderbuffer(n.FRAMEBUFFER,K,n.RENDERBUFFER,k)}}else if(e.bindFramebuffer(n.FRAMEBUFFER,v.__webglFramebuffer),v.__webglDepthbuffer===void 0)v.__webglDepthbuffer=n.createRenderbuffer(),Mt(v.__webglDepthbuffer,M,!1);else{let H=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,K=v.__webglDepthbuffer;n.bindRenderbuffer(n.RENDERBUFFER,K),n.framebufferRenderbuffer(n.FRAMEBUFFER,H,n.RENDERBUFFER,K)}e.bindFramebuffer(n.FRAMEBUFFER,null)}function It(M,v,P){let H=i.get(M);v!==void 0&&st(H.__webglFramebuffer,M,M.texture,n.COLOR_ATTACHMENT0,n.TEXTURE_2D,0),P!==void 0&&Nt(M)}function kt(M){let v=M.texture,P=i.get(M),H=i.get(v);M.addEventListener("dispose",T);let K=M.textures,k=M.isWebGLCubeRenderTarget===!0,nt=K.length>1;if(nt||(H.__webglTexture===void 0&&(H.__webglTexture=n.createTexture()),H.__version=v.version,o.memory.textures++),k){P.__webglFramebuffer=[];for(let Q=0;Q<6;Q++)if(v.mipmaps&&v.mipmaps.length>0){P.__webglFramebuffer[Q]=[];for(let X=0;X<v.mipmaps.length;X++)P.__webglFramebuffer[Q][X]=n.createFramebuffer()}else P.__webglFramebuffer[Q]=n.createFramebuffer()}else{if(v.mipmaps&&v.mipmaps.length>0){P.__webglFramebuffer=[];for(let Q=0;Q<v.mipmaps.length;Q++)P.__webglFramebuffer[Q]=n.createFramebuffer()}else P.__webglFramebuffer=n.createFramebuffer();if(nt)for(let Q=0,X=K.length;Q<X;Q++){let ht=i.get(K[Q]);ht.__webglTexture===void 0&&(ht.__webglTexture=n.createTexture(),o.memory.textures++)}if(M.samples>0&&ct(M)===!1){P.__webglMultisampledFramebuffer=n.createFramebuffer(),P.__webglColorRenderbuffer=[],e.bindFramebuffer(n.FRAMEBUFFER,P.__webglMultisampledFramebuffer);for(let Q=0;Q<K.length;Q++){let X=K[Q];P.__webglColorRenderbuffer[Q]=n.createRenderbuffer(),n.bindRenderbuffer(n.RENDERBUFFER,P.__webglColorRenderbuffer[Q]);let ht=r.convert(X.format,X.colorSpace),tt=r.convert(X.type),lt=b(X.internalFormat,ht,tt,X.colorSpace,M.isXRRenderTarget===!0),At=j(M);n.renderbufferStorageMultisample(n.RENDERBUFFER,At,lt,M.width,M.height),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+Q,n.RENDERBUFFER,P.__webglColorRenderbuffer[Q])}n.bindRenderbuffer(n.RENDERBUFFER,null),M.depthBuffer&&(P.__webglDepthRenderbuffer=n.createRenderbuffer(),Mt(P.__webglDepthRenderbuffer,M,!0)),e.bindFramebuffer(n.FRAMEBUFFER,null)}}if(k){e.bindTexture(n.TEXTURE_CUBE_MAP,H.__webglTexture),bt(n.TEXTURE_CUBE_MAP,v);for(let Q=0;Q<6;Q++)if(v.mipmaps&&v.mipmaps.length>0)for(let X=0;X<v.mipmaps.length;X++)st(P.__webglFramebuffer[Q][X],M,v,n.COLOR_ATTACHMENT0,n.TEXTURE_CUBE_MAP_POSITIVE_X+Q,X);else st(P.__webglFramebuffer[Q],M,v,n.COLOR_ATTACHMENT0,n.TEXTURE_CUBE_MAP_POSITIVE_X+Q,0);u(v)&&p(n.TEXTURE_CUBE_MAP),e.unbindTexture()}else if(nt){for(let Q=0,X=K.length;Q<X;Q++){let ht=K[Q],tt=i.get(ht);e.bindTexture(n.TEXTURE_2D,tt.__webglTexture),bt(n.TEXTURE_2D,ht),st(P.__webglFramebuffer,M,ht,n.COLOR_ATTACHMENT0+Q,n.TEXTURE_2D,0),u(ht)&&p(n.TEXTURE_2D)}e.unbindTexture()}else{let Q=n.TEXTURE_2D;if((M.isWebGL3DRenderTarget||M.isWebGLArrayRenderTarget)&&(Q=M.isWebGL3DRenderTarget?n.TEXTURE_3D:n.TEXTURE_2D_ARRAY),e.bindTexture(Q,H.__webglTexture),bt(Q,v),v.mipmaps&&v.mipmaps.length>0)for(let X=0;X<v.mipmaps.length;X++)st(P.__webglFramebuffer[X],M,v,n.COLOR_ATTACHMENT0,Q,X);else st(P.__webglFramebuffer,M,v,n.COLOR_ATTACHMENT0,Q,0);u(v)&&p(Q),e.unbindTexture()}M.depthBuffer&&Nt(M)}function Zt(M){let v=M.textures;for(let P=0,H=v.length;P<H;P++){let K=v[P];if(u(K)){let k=M.isWebGLCubeRenderTarget?n.TEXTURE_CUBE_MAP:n.TEXTURE_2D,nt=i.get(K).__webglTexture;e.bindTexture(k,nt),p(k),e.unbindTexture()}}}let Vt=[],I=[];function me(M){if(M.samples>0){if(ct(M)===!1){let v=M.textures,P=M.width,H=M.height,K=n.COLOR_BUFFER_BIT,k=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,nt=i.get(M),Q=v.length>1;if(Q)for(let X=0;X<v.length;X++)e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglMultisampledFramebuffer),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.RENDERBUFFER,null),e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglFramebuffer),n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.TEXTURE_2D,null,0);e.bindFramebuffer(n.READ_FRAMEBUFFER,nt.__webglMultisampledFramebuffer),e.bindFramebuffer(n.DRAW_FRAMEBUFFER,nt.__webglFramebuffer);for(let X=0;X<v.length;X++){if(M.resolveDepthBuffer&&(M.depthBuffer&&(K|=n.DEPTH_BUFFER_BIT),M.stencilBuffer&&M.resolveStencilBuffer&&(K|=n.STENCIL_BUFFER_BIT)),Q){n.framebufferRenderbuffer(n.READ_FRAMEBUFFER,n.COLOR_ATTACHMENT0,n.RENDERBUFFER,nt.__webglColorRenderbuffer[X]);let ht=i.get(v[X]).__webglTexture;n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0,n.TEXTURE_2D,ht,0)}n.blitFramebuffer(0,0,P,H,0,0,P,H,K,n.NEAREST),c===!0&&(Vt.length=0,I.length=0,Vt.push(n.COLOR_ATTACHMENT0+X),M.depthBuffer&&M.resolveDepthBuffer===!1&&(Vt.push(k),I.push(k),n.invalidateFramebuffer(n.DRAW_FRAMEBUFFER,I)),n.invalidateFramebuffer(n.READ_FRAMEBUFFER,Vt))}if(e.bindFramebuffer(n.READ_FRAMEBUFFER,null),e.bindFramebuffer(n.DRAW_FRAMEBUFFER,null),Q)for(let X=0;X<v.length;X++){e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglMultisampledFramebuffer),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.RENDERBUFFER,nt.__webglColorRenderbuffer[X]);let ht=i.get(v[X]).__webglTexture;e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglFramebuffer),n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.TEXTURE_2D,ht,0)}e.bindFramebuffer(n.DRAW_FRAMEBUFFER,nt.__webglMultisampledFramebuffer)}else if(M.depthBuffer&&M.resolveDepthBuffer===!1&&c){let v=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT;n.invalidateFramebuffer(n.DRAW_FRAMEBUFFER,[v])}}}function j(M){return Math.min(s.maxSamples,M.samples)}function ct(M){let v=i.get(M);return M.samples>0&&t.has("WEBGL_multisampled_render_to_texture")===!0&&v.__useRenderToTexture!==!1}function ut(M){let v=o.render.frame;f.get(M)!==v&&(f.set(M,v),M.update())}function Rt(M,v){let P=M.colorSpace,H=M.format,K=M.type;return M.isCompressedTexture===!0||M.isVideoTexture===!0||P!==si&&P!==Qn&&(Jt.getTransfer(P)===ie?(H!==fn||K!==Fn)&&console.warn("THREE.WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):console.error("THREE.WebGLTextures: Unsupported texture color space:",P)),v}function z(M){return typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement?(l.width=M.naturalWidth||M.width,l.height=M.naturalHeight||M.height):typeof VideoFrame<"u"&&M instanceof VideoFrame?(l.width=M.displayWidth,l.height=M.displayHeight):(l.width=M.width,l.height=M.height),l}this.allocateTextureUnit=F,this.resetTextureUnits=S,this.setTexture2D=q,this.setTexture2DArray=et,this.setTexture3D=B,this.setTextureCube=it,this.rebindTextures=It,this.setupRenderTarget=kt,this.updateRenderTargetMipmap=Zt,this.updateMultisampleRenderTarget=me,this.setupDepthRenderbuffer=Nt,this.setupFrameBufferTexture=st,this.useMultisampledRTT=ct}function yv(n,t){function e(i,s=Qn){let r,o=Jt.getTransfer(s);if(i===Fn)return n.UNSIGNED_BYTE;if(i===Nl)return n.UNSIGNED_SHORT_4_4_4_4;if(i===Ol)return n.UNSIGNED_SHORT_5_5_5_1;if(i===Xu)return n.UNSIGNED_INT_5_9_9_9_REV;if(i===Gu)return n.BYTE;if(i===Wu)return n.SHORT;if(i===ar)return n.UNSIGNED_SHORT;if(i===Ul)return n.INT;if(i===Ti)return n.UNSIGNED_INT;if(i===bn)return n.FLOAT;if(i===Ve)return n.HALF_FLOAT;if(i===qu)return n.ALPHA;if(i===Yu)return n.RGB;if(i===fn)return n.RGBA;if(i===Zu)return n.LUMINANCE;if(i===ju)return n.LUMINANCE_ALPHA;if(i===ps)return n.DEPTH_COMPONENT;if(i===Ms)return n.DEPTH_STENCIL;if(i===Fl)return n.RED;if(i===Bl)return n.RED_INTEGER;if(i===Ku)return n.RG;if(i===zl)return n.RG_INTEGER;if(i===kl)return n.RGBA_INTEGER;if(i===ao||i===co||i===lo||i===ho)if(o===ie)if(r=t.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(i===ao)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(i===co)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(i===lo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(i===ho)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=t.get("WEBGL_compressed_texture_s3tc"),r!==null){if(i===ao)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(i===co)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(i===lo)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(i===ho)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(i===Pc||i===Ic||i===Lc||i===Dc)if(r=t.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(i===Pc)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(i===Ic)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(i===Lc)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(i===Dc)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(i===Uc||i===Nc||i===Oc)if(r=t.get("WEBGL_compressed_texture_etc"),r!==null){if(i===Uc||i===Nc)return o===ie?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(i===Oc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC}else return null;if(i===Fc||i===Bc||i===zc||i===kc||i===Hc||i===Vc||i===Gc||i===Wc||i===Xc||i===qc||i===Yc||i===Zc||i===jc||i===Kc)if(r=t.get("WEBGL_compressed_texture_astc"),r!==null){if(i===Fc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(i===Bc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(i===zc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(i===kc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(i===Hc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(i===Vc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(i===Gc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(i===Wc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(i===Xc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(i===qc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(i===Yc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(i===Zc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(i===jc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(i===Kc)return o===ie?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(i===uo||i===Jc||i===Qc)if(r=t.get("EXT_texture_compression_bptc"),r!==null){if(i===uo)return o===ie?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(i===Jc)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(i===Qc)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(i===Ju||i===$c||i===tl||i===el)if(r=t.get("EXT_texture_compression_rgtc"),r!==null){if(i===uo)return r.COMPRESSED_RED_RGTC1_EXT;if(i===$c)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(i===tl)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(i===el)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return i===ys?n.UNSIGNED_INT_24_8:n[i]!==void 0?n[i]:null}return{convert:e}}var gl=class extends Ne{constructor(t=[]){super(),this.isArrayCamera=!0,this.cameras=t}},$n=class extends He{constructor(){super(),this.isGroup=!0,this.type="Group"}},Mv={type:"move"},or=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new $n,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new $n,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new D,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new D),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new $n,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new D,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new D),this._grip}dispatchEvent(t){return this._targetRay!==null&&this._targetRay.dispatchEvent(t),this._grip!==null&&this._grip.dispatchEvent(t),this._hand!==null&&this._hand.dispatchEvent(t),this}connect(t){if(t&&t.hand){let e=this._hand;if(e)for(let i of t.hand.values())this._getHandJoint(e,i)}return this.dispatchEvent({type:"connected",data:t}),this}disconnect(t){return this.dispatchEvent({type:"disconnected",data:t}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(t,e,i){let s=null,r=null,o=null,a=this._targetRay,c=this._grip,l=this._hand;if(t&&e.session.visibilityState!=="visible-blurred"){if(l&&t.hand){o=!0;for(let _ of t.hand.values()){let u=e.getJointPose(_,i),p=this._getHandJoint(l,_);u!==null&&(p.matrix.fromArray(u.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=u.radius),p.visible=u!==null}let f=l.joints["index-finger-tip"],d=l.joints["thumb-tip"],h=f.position.distanceTo(d.position),m=.02,x=.005;l.inputState.pinching&&h>m+x?(l.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:t.handedness,target:this})):!l.inputState.pinching&&h<=m-x&&(l.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:t.handedness,target:this}))}else c!==null&&t.gripSpace&&(r=e.getPose(t.gripSpace,i),r!==null&&(c.matrix.fromArray(r.transform.matrix),c.matrix.decompose(c.position,c.rotation,c.scale),c.matrixWorldNeedsUpdate=!0,r.linearVelocity?(c.hasLinearVelocity=!0,c.linearVelocity.copy(r.linearVelocity)):c.hasLinearVelocity=!1,r.angularVelocity?(c.hasAngularVelocity=!0,c.angularVelocity.copy(r.angularVelocity)):c.hasAngularVelocity=!1));a!==null&&(s=e.getPose(t.targetRaySpace,i),s===null&&r!==null&&(s=r),s!==null&&(a.matrix.fromArray(s.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,s.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(s.linearVelocity)):a.hasLinearVelocity=!1,s.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(s.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(Mv)))}return a!==null&&(a.visible=s!==null),c!==null&&(c.visible=r!==null),l!==null&&(l.visible=o!==null),this}_getHandJoint(t,e){if(t.joints[e.jointName]===void 0){let i=new $n;i.matrixAutoUpdate=!1,i.visible=!1,t.joints[e.jointName]=i,t.add(i)}return t.joints[e.jointName]}},bv=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Sv=`
uniform sampler2DArray depthColor;
uniform float depthWidth;
uniform float depthHeight;

void main() {

	vec2 coord = vec2( gl_FragCoord.x / depthWidth, gl_FragCoord.y / depthHeight );

	if ( coord.x >= 1.0 ) {

		gl_FragDepth = texture( depthColor, vec3( coord.x - 1.0, coord.y, 1 ) ).r;

	} else {

		gl_FragDepth = texture( depthColor, vec3( coord.x, coord.y, 0 ) ).r;

	}

}`,xl=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(t,e,i){if(this.texture===null){let s=new Pe,r=t.properties.get(s);r.__webglTexture=e.texture,(e.depthNear!=i.depthNear||e.depthFar!=i.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=s}}getMesh(t){if(this.texture!==null&&this.mesh===null){let e=t.cameras[0].viewport,i=new te({vertexShader:bv,fragmentShader:Sv,uniforms:{depthColor:{value:this.texture},depthWidth:{value:e.z},depthHeight:{value:e.w}}});this.mesh=new Oe(new To(20,20),i)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},vl=class extends Bn{constructor(t,e){super();let i=this,s=null,r=1,o=null,a="local-floor",c=1,l=null,f=null,d=null,h=null,m=null,x=null,_=new xl,u=e.getContextAttributes(),p=null,b=null,y=[],A=[],R=new yt,T=null,E=new Ne;E.layers.enable(1),E.viewport=new ce;let C=new Ne;C.layers.enable(2),C.viewport=new ce;let N=[E,C],g=new gl;g.layers.enable(1),g.layers.enable(2);let S=null,F=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(J){let st=y[J];return st===void 0&&(st=new or,y[J]=st),st.getTargetRaySpace()},this.getControllerGrip=function(J){let st=y[J];return st===void 0&&(st=new or,y[J]=st),st.getGripSpace()},this.getHand=function(J){let st=y[J];return st===void 0&&(st=new or,y[J]=st),st.getHandSpace()};function V(J){let st=A.indexOf(J.inputSource);if(st===-1)return;let Mt=y[st];Mt!==void 0&&(Mt.update(J.inputSource,J.frame,l||o),Mt.dispatchEvent({type:J.type,data:J.inputSource}))}function q(){s.removeEventListener("select",V),s.removeEventListener("selectstart",V),s.removeEventListener("selectend",V),s.removeEventListener("squeeze",V),s.removeEventListener("squeezestart",V),s.removeEventListener("squeezeend",V),s.removeEventListener("end",q),s.removeEventListener("inputsourceschange",et);for(let J=0;J<y.length;J++){let st=A[J];st!==null&&(A[J]=null,y[J].disconnect(st))}S=null,F=null,_.reset(),t.setRenderTarget(p),m=null,h=null,d=null,s=null,b=null,Wt.stop(),i.isPresenting=!1,t.setPixelRatio(T),t.setSize(R.width,R.height,!1),i.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(J){r=J,i.isPresenting===!0&&console.warn("THREE.WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(J){a=J,i.isPresenting===!0&&console.warn("THREE.WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return l||o},this.setReferenceSpace=function(J){l=J},this.getBaseLayer=function(){return h!==null?h:m},this.getBinding=function(){return d},this.getFrame=function(){return x},this.getSession=function(){return s},this.setSession=async function(J){if(s=J,s!==null){if(p=t.getRenderTarget(),s.addEventListener("select",V),s.addEventListener("selectstart",V),s.addEventListener("selectend",V),s.addEventListener("squeeze",V),s.addEventListener("squeezestart",V),s.addEventListener("squeezeend",V),s.addEventListener("end",q),s.addEventListener("inputsourceschange",et),u.xrCompatible!==!0&&await e.makeXRCompatible(),T=t.getPixelRatio(),t.getSize(R),s.renderState.layers===void 0){let st={antialias:u.antialias,alpha:!0,depth:u.depth,stencil:u.stencil,framebufferScaleFactor:r};m=new XRWebGLLayer(s,e,st),s.updateRenderState({baseLayer:m}),t.setPixelRatio(1),t.setSize(m.framebufferWidth,m.framebufferHeight,!1),b=new fe(m.framebufferWidth,m.framebufferHeight,{format:fn,type:Fn,colorSpace:t.outputColorSpace,stencilBuffer:u.stencil})}else{let st=null,Mt=null,xt=null;u.depth&&(xt=u.stencil?e.DEPTH24_STENCIL8:e.DEPTH_COMPONENT24,st=u.stencil?Ms:ps,Mt=u.stencil?ys:Ti);let Nt={colorFormat:e.RGBA8,depthFormat:xt,scaleFactor:r};d=new XRWebGLBinding(s,e),h=d.createProjectionLayer(Nt),s.updateRenderState({layers:[h]}),t.setPixelRatio(1),t.setSize(h.textureWidth,h.textureHeight,!1),b=new fe(h.textureWidth,h.textureHeight,{format:fn,type:Fn,depthTexture:new Ro(h.textureWidth,h.textureHeight,Mt,void 0,void 0,void 0,void 0,void 0,void 0,st),stencilBuffer:u.stencil,colorSpace:t.outputColorSpace,samples:u.antialias?4:0,resolveDepthBuffer:h.ignoreDepthValues===!1})}b.isXRRenderTarget=!0,this.setFoveation(c),l=null,o=await s.requestReferenceSpace(a),Wt.setContext(s),Wt.start(),i.isPresenting=!0,i.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(s!==null)return s.environmentBlendMode},this.getDepthTexture=function(){return _.getDepthTexture()};function et(J){for(let st=0;st<J.removed.length;st++){let Mt=J.removed[st],xt=A.indexOf(Mt);xt>=0&&(A[xt]=null,y[xt].disconnect(Mt))}for(let st=0;st<J.added.length;st++){let Mt=J.added[st],xt=A.indexOf(Mt);if(xt===-1){for(let It=0;It<y.length;It++)if(It>=A.length){A.push(Mt),xt=It;break}else if(A[It]===null){A[It]=Mt,xt=It;break}if(xt===-1)break}let Nt=y[xt];Nt&&Nt.connect(Mt)}}let B=new D,it=new D;function Z(J,st,Mt){B.setFromMatrixPosition(st.matrixWorld),it.setFromMatrixPosition(Mt.matrixWorld);let xt=B.distanceTo(it),Nt=st.projectionMatrix.elements,It=Mt.projectionMatrix.elements,kt=Nt[14]/(Nt[10]-1),Zt=Nt[14]/(Nt[10]+1),Vt=(Nt[9]+1)/Nt[5],I=(Nt[9]-1)/Nt[5],me=(Nt[8]-1)/Nt[0],j=(It[8]+1)/It[0],ct=kt*me,ut=kt*j,Rt=xt/(-me+j),z=Rt*-me;if(st.matrixWorld.decompose(J.position,J.quaternion,J.scale),J.translateX(z),J.translateZ(Rt),J.matrixWorld.compose(J.position,J.quaternion,J.scale),J.matrixWorldInverse.copy(J.matrixWorld).invert(),Nt[10]===-1)J.projectionMatrix.copy(st.projectionMatrix),J.projectionMatrixInverse.copy(st.projectionMatrixInverse);else{let M=kt+Rt,v=Zt+Rt,P=ct-z,H=ut+(xt-z),K=Vt*Zt/v*M,k=I*Zt/v*M;J.projectionMatrix.makePerspective(P,H,K,k,M,v),J.projectionMatrixInverse.copy(J.projectionMatrix).invert()}}function gt(J,st){st===null?J.matrixWorld.copy(J.matrix):J.matrixWorld.multiplyMatrices(st.matrixWorld,J.matrix),J.matrixWorldInverse.copy(J.matrixWorld).invert()}this.updateCamera=function(J){if(s===null)return;let st=J.near,Mt=J.far;_.texture!==null&&(_.depthNear>0&&(st=_.depthNear),_.depthFar>0&&(Mt=_.depthFar)),g.near=C.near=E.near=st,g.far=C.far=E.far=Mt,(S!==g.near||F!==g.far)&&(s.updateRenderState({depthNear:g.near,depthFar:g.far}),S=g.near,F=g.far);let xt=J.parent,Nt=g.cameras;gt(g,xt);for(let It=0;It<Nt.length;It++)gt(Nt[It],xt);Nt.length===2?Z(g,E,C):g.projectionMatrix.copy(E.projectionMatrix),_t(J,g,xt)};function _t(J,st,Mt){Mt===null?J.matrix.copy(st.matrixWorld):(J.matrix.copy(Mt.matrixWorld),J.matrix.invert(),J.matrix.multiply(st.matrixWorld)),J.matrix.decompose(J.position,J.quaternion,J.scale),J.updateMatrixWorld(!0),J.projectionMatrix.copy(st.projectionMatrix),J.projectionMatrixInverse.copy(st.projectionMatrixInverse),J.isPerspectiveCamera&&(J.fov=cr*2*Math.atan(1/J.projectionMatrix.elements[5]),J.zoom=1)}this.getCamera=function(){return g},this.getFoveation=function(){if(!(h===null&&m===null))return c},this.setFoveation=function(J){c=J,h!==null&&(h.fixedFoveation=J),m!==null&&m.fixedFoveation!==void 0&&(m.fixedFoveation=J)},this.hasDepthSensing=function(){return _.texture!==null},this.getDepthSensingMesh=function(){return _.getMesh(g)};let bt=null;function Gt(J,st){if(f=st.getViewerPose(l||o),x=st,f!==null){let Mt=f.views;m!==null&&(t.setRenderTargetFramebuffer(b,m.framebuffer),t.setRenderTarget(b));let xt=!1;Mt.length!==g.cameras.length&&(g.cameras.length=0,xt=!0);for(let It=0;It<Mt.length;It++){let kt=Mt[It],Zt=null;if(m!==null)Zt=m.getViewport(kt);else{let I=d.getViewSubImage(h,kt);Zt=I.viewport,It===0&&(t.setRenderTargetTextures(b,I.colorTexture,h.ignoreDepthValues?void 0:I.depthStencilTexture),t.setRenderTarget(b))}let Vt=N[It];Vt===void 0&&(Vt=new Ne,Vt.layers.enable(It),Vt.viewport=new ce,N[It]=Vt),Vt.matrix.fromArray(kt.transform.matrix),Vt.matrix.decompose(Vt.position,Vt.quaternion,Vt.scale),Vt.projectionMatrix.fromArray(kt.projectionMatrix),Vt.projectionMatrixInverse.copy(Vt.projectionMatrix).invert(),Vt.viewport.set(Zt.x,Zt.y,Zt.width,Zt.height),It===0&&(g.matrix.copy(Vt.matrix),g.matrix.decompose(g.position,g.quaternion,g.scale)),xt===!0&&g.cameras.push(Vt)}let Nt=s.enabledFeatures;if(Nt&&Nt.includes("depth-sensing")){let It=d.getDepthInformation(Mt[0]);It&&It.isValid&&It.texture&&_.init(t,It,s.renderState)}}for(let Mt=0;Mt<y.length;Mt++){let xt=A[Mt],Nt=y[Mt];xt!==null&&Nt!==void 0&&Nt.update(xt,st,l||o)}bt&&bt(J,st),st.detectedPlanes&&i.dispatchEvent({type:"planesdetected",data:st}),x=null}let Wt=new id;Wt.setAnimationLoop(Gt),this.setAnimationLoop=function(J){bt=J},this.dispose=function(){}}},yi=new nn,wv=new $t;function Av(n,t){function e(u,p){u.matrixAutoUpdate===!0&&u.updateMatrix(),p.value.copy(u.matrix)}function i(u,p){p.color.getRGB(u.fogColor.value,nd(n)),p.isFog?(u.fogNear.value=p.near,u.fogFar.value=p.far):p.isFogExp2&&(u.fogDensity.value=p.density)}function s(u,p,b,y,A){p.isMeshBasicMaterial||p.isMeshLambertMaterial?r(u,p):p.isMeshToonMaterial?(r(u,p),d(u,p)):p.isMeshPhongMaterial?(r(u,p),f(u,p)):p.isMeshStandardMaterial?(r(u,p),h(u,p),p.isMeshPhysicalMaterial&&m(u,p,A)):p.isMeshMatcapMaterial?(r(u,p),x(u,p)):p.isMeshDepthMaterial?r(u,p):p.isMeshDistanceMaterial?(r(u,p),_(u,p)):p.isMeshNormalMaterial?r(u,p):p.isLineBasicMaterial?(o(u,p),p.isLineDashedMaterial&&a(u,p)):p.isPointsMaterial?c(u,p,b,y):p.isSpriteMaterial?l(u,p):p.isShadowMaterial?(u.color.value.copy(p.color),u.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(u,p){u.opacity.value=p.opacity,p.color&&u.diffuse.value.copy(p.color),p.emissive&&u.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(u.map.value=p.map,e(p.map,u.mapTransform)),p.alphaMap&&(u.alphaMap.value=p.alphaMap,e(p.alphaMap,u.alphaMapTransform)),p.bumpMap&&(u.bumpMap.value=p.bumpMap,e(p.bumpMap,u.bumpMapTransform),u.bumpScale.value=p.bumpScale,p.side===ze&&(u.bumpScale.value*=-1)),p.normalMap&&(u.normalMap.value=p.normalMap,e(p.normalMap,u.normalMapTransform),u.normalScale.value.copy(p.normalScale),p.side===ze&&u.normalScale.value.negate()),p.displacementMap&&(u.displacementMap.value=p.displacementMap,e(p.displacementMap,u.displacementMapTransform),u.displacementScale.value=p.displacementScale,u.displacementBias.value=p.displacementBias),p.emissiveMap&&(u.emissiveMap.value=p.emissiveMap,e(p.emissiveMap,u.emissiveMapTransform)),p.specularMap&&(u.specularMap.value=p.specularMap,e(p.specularMap,u.specularMapTransform)),p.alphaTest>0&&(u.alphaTest.value=p.alphaTest);let b=t.get(p),y=b.envMap,A=b.envMapRotation;y&&(u.envMap.value=y,yi.copy(A),yi.x*=-1,yi.y*=-1,yi.z*=-1,y.isCubeTexture&&y.isRenderTargetTexture===!1&&(yi.y*=-1,yi.z*=-1),u.envMapRotation.value.setFromMatrix4(wv.makeRotationFromEuler(yi)),u.flipEnvMap.value=y.isCubeTexture&&y.isRenderTargetTexture===!1?-1:1,u.reflectivity.value=p.reflectivity,u.ior.value=p.ior,u.refractionRatio.value=p.refractionRatio),p.lightMap&&(u.lightMap.value=p.lightMap,u.lightMapIntensity.value=p.lightMapIntensity,e(p.lightMap,u.lightMapTransform)),p.aoMap&&(u.aoMap.value=p.aoMap,u.aoMapIntensity.value=p.aoMapIntensity,e(p.aoMap,u.aoMapTransform))}function o(u,p){u.diffuse.value.copy(p.color),u.opacity.value=p.opacity,p.map&&(u.map.value=p.map,e(p.map,u.mapTransform))}function a(u,p){u.dashSize.value=p.dashSize,u.totalSize.value=p.dashSize+p.gapSize,u.scale.value=p.scale}function c(u,p,b,y){u.diffuse.value.copy(p.color),u.opacity.value=p.opacity,u.size.value=p.size*b,u.scale.value=y*.5,p.map&&(u.map.value=p.map,e(p.map,u.uvTransform)),p.alphaMap&&(u.alphaMap.value=p.alphaMap,e(p.alphaMap,u.alphaMapTransform)),p.alphaTest>0&&(u.alphaTest.value=p.alphaTest)}function l(u,p){u.diffuse.value.copy(p.color),u.opacity.value=p.opacity,u.rotation.value=p.rotation,p.map&&(u.map.value=p.map,e(p.map,u.mapTransform)),p.alphaMap&&(u.alphaMap.value=p.alphaMap,e(p.alphaMap,u.alphaMapTransform)),p.alphaTest>0&&(u.alphaTest.value=p.alphaTest)}function f(u,p){u.specular.value.copy(p.specular),u.shininess.value=Math.max(p.shininess,1e-4)}function d(u,p){p.gradientMap&&(u.gradientMap.value=p.gradientMap)}function h(u,p){u.metalness.value=p.metalness,p.metalnessMap&&(u.metalnessMap.value=p.metalnessMap,e(p.metalnessMap,u.metalnessMapTransform)),u.roughness.value=p.roughness,p.roughnessMap&&(u.roughnessMap.value=p.roughnessMap,e(p.roughnessMap,u.roughnessMapTransform)),p.envMap&&(u.envMapIntensity.value=p.envMapIntensity)}function m(u,p,b){u.ior.value=p.ior,p.sheen>0&&(u.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),u.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(u.sheenColorMap.value=p.sheenColorMap,e(p.sheenColorMap,u.sheenColorMapTransform)),p.sheenRoughnessMap&&(u.sheenRoughnessMap.value=p.sheenRoughnessMap,e(p.sheenRoughnessMap,u.sheenRoughnessMapTransform))),p.clearcoat>0&&(u.clearcoat.value=p.clearcoat,u.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(u.clearcoatMap.value=p.clearcoatMap,e(p.clearcoatMap,u.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(u.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,e(p.clearcoatRoughnessMap,u.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(u.clearcoatNormalMap.value=p.clearcoatNormalMap,e(p.clearcoatNormalMap,u.clearcoatNormalMapTransform),u.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===ze&&u.clearcoatNormalScale.value.negate())),p.dispersion>0&&(u.dispersion.value=p.dispersion),p.iridescence>0&&(u.iridescence.value=p.iridescence,u.iridescenceIOR.value=p.iridescenceIOR,u.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],u.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(u.iridescenceMap.value=p.iridescenceMap,e(p.iridescenceMap,u.iridescenceMapTransform)),p.iridescenceThicknessMap&&(u.iridescenceThicknessMap.value=p.iridescenceThicknessMap,e(p.iridescenceThicknessMap,u.iridescenceThicknessMapTransform))),p.transmission>0&&(u.transmission.value=p.transmission,u.transmissionSamplerMap.value=b.texture,u.transmissionSamplerSize.value.set(b.width,b.height),p.transmissionMap&&(u.transmissionMap.value=p.transmissionMap,e(p.transmissionMap,u.transmissionMapTransform)),u.thickness.value=p.thickness,p.thicknessMap&&(u.thicknessMap.value=p.thicknessMap,e(p.thicknessMap,u.thicknessMapTransform)),u.attenuationDistance.value=p.attenuationDistance,u.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(u.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(u.anisotropyMap.value=p.anisotropyMap,e(p.anisotropyMap,u.anisotropyMapTransform))),u.specularIntensity.value=p.specularIntensity,u.specularColor.value.copy(p.specularColor),p.specularColorMap&&(u.specularColorMap.value=p.specularColorMap,e(p.specularColorMap,u.specularColorMapTransform)),p.specularIntensityMap&&(u.specularIntensityMap.value=p.specularIntensityMap,e(p.specularIntensityMap,u.specularIntensityMapTransform))}function x(u,p){p.matcap&&(u.matcap.value=p.matcap)}function _(u,p){let b=t.get(p).light;u.referencePosition.value.setFromMatrixPosition(b.matrixWorld),u.nearDistance.value=b.shadow.camera.near,u.farDistance.value=b.shadow.camera.far}return{refreshFogUniforms:i,refreshMaterialUniforms:s}}function Ev(n,t,e,i){let s={},r={},o=[],a=n.getParameter(n.MAX_UNIFORM_BUFFER_BINDINGS);function c(b,y){let A=y.program;i.uniformBlockBinding(b,A)}function l(b,y){let A=s[b.id];A===void 0&&(x(b),A=f(b),s[b.id]=A,b.addEventListener("dispose",u));let R=y.program;i.updateUBOMapping(b,R);let T=t.render.frame;r[b.id]!==T&&(h(b),r[b.id]=T)}function f(b){let y=d();b.__bindingPointIndex=y;let A=n.createBuffer(),R=b.__size,T=b.usage;return n.bindBuffer(n.UNIFORM_BUFFER,A),n.bufferData(n.UNIFORM_BUFFER,R,T),n.bindBuffer(n.UNIFORM_BUFFER,null),n.bindBufferBase(n.UNIFORM_BUFFER,y,A),A}function d(){for(let b=0;b<a;b++)if(o.indexOf(b)===-1)return o.push(b),b;return console.error("THREE.WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function h(b){let y=s[b.id],A=b.uniforms,R=b.__cache;n.bindBuffer(n.UNIFORM_BUFFER,y);for(let T=0,E=A.length;T<E;T++){let C=Array.isArray(A[T])?A[T]:[A[T]];for(let N=0,g=C.length;N<g;N++){let S=C[N];if(m(S,T,N,R)===!0){let F=S.__offset,V=Array.isArray(S.value)?S.value:[S.value],q=0;for(let et=0;et<V.length;et++){let B=V[et],it=_(B);typeof B=="number"||typeof B=="boolean"?(S.__data[0]=B,n.bufferSubData(n.UNIFORM_BUFFER,F+q,S.__data)):B.isMatrix3?(S.__data[0]=B.elements[0],S.__data[1]=B.elements[1],S.__data[2]=B.elements[2],S.__data[3]=0,S.__data[4]=B.elements[3],S.__data[5]=B.elements[4],S.__data[6]=B.elements[5],S.__data[7]=0,S.__data[8]=B.elements[6],S.__data[9]=B.elements[7],S.__data[10]=B.elements[8],S.__data[11]=0):(B.toArray(S.__data,q),q+=it.storage/Float32Array.BYTES_PER_ELEMENT)}n.bufferSubData(n.UNIFORM_BUFFER,F,S.__data)}}}n.bindBuffer(n.UNIFORM_BUFFER,null)}function m(b,y,A,R){let T=b.value,E=y+"_"+A;if(R[E]===void 0)return typeof T=="number"||typeof T=="boolean"?R[E]=T:R[E]=T.clone(),!0;{let C=R[E];if(typeof T=="number"||typeof T=="boolean"){if(C!==T)return R[E]=T,!0}else if(C.equals(T)===!1)return C.copy(T),!0}return!1}function x(b){let y=b.uniforms,A=0,R=16;for(let E=0,C=y.length;E<C;E++){let N=Array.isArray(y[E])?y[E]:[y[E]];for(let g=0,S=N.length;g<S;g++){let F=N[g],V=Array.isArray(F.value)?F.value:[F.value];for(let q=0,et=V.length;q<et;q++){let B=V[q],it=_(B),Z=A%R,gt=Z%it.boundary,_t=Z+gt;A+=gt,_t!==0&&R-_t<it.storage&&(A+=R-_t),F.__data=new Float32Array(it.storage/Float32Array.BYTES_PER_ELEMENT),F.__offset=A,A+=it.storage}}}let T=A%R;return T>0&&(A+=R-T),b.__size=A,b.__cache={},this}function _(b){let y={boundary:0,storage:0};return typeof b=="number"||typeof b=="boolean"?(y.boundary=4,y.storage=4):b.isVector2?(y.boundary=8,y.storage=8):b.isVector3||b.isColor?(y.boundary=16,y.storage=12):b.isVector4?(y.boundary=16,y.storage=16):b.isMatrix3?(y.boundary=48,y.storage=48):b.isMatrix4?(y.boundary=64,y.storage=64):b.isTexture?console.warn("THREE.WebGLRenderer: Texture samplers can not be part of an uniforms group."):console.warn("THREE.WebGLRenderer: Unsupported uniform value type.",b),y}function u(b){let y=b.target;y.removeEventListener("dispose",u);let A=o.indexOf(y.__bindingPointIndex);o.splice(A,1),n.deleteBuffer(s[y.id]),delete s[y.id],delete r[y.id]}function p(){for(let b in s)n.deleteBuffer(s[b]);o=[],s={},r={}}return{bind:c,update:l,dispose:p}}var Po=class{constructor(t={}){let{canvas:e=Pp(),context:i=null,depth:s=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:c=!0,preserveDrawingBuffer:l=!1,powerPreference:f="default",failIfMajorPerformanceCaveat:d=!1}=t;this.isWebGLRenderer=!0;let h;if(i!==null){if(typeof WebGLRenderingContext<"u"&&i instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");h=i.getContextAttributes().alpha}else h=o;let m=new Uint32Array(4),x=new Int32Array(4),_=null,u=null,p=[],b=[];this.domElement=e,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this._outputColorSpace=yn,this.toneMapping=ti,this.toneMappingExposure=1;let y=this,A=!1,R=0,T=0,E=null,C=-1,N=null,g=new ce,S=new ce,F=null,V=new Ot(0),q=0,et=e.width,B=e.height,it=1,Z=null,gt=null,_t=new ce(0,0,et,B),bt=new ce(0,0,et,B),Gt=!1,Wt=new ur,J=!1,st=!1,Mt=new $t,xt=new $t,Nt=new D,It=new ce,kt={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Zt=!1;function Vt(){return E===null?it:1}let I=i;function me(w,U){return e.getContext(w,U)}try{let w={alpha:!0,depth:s,stencil:r,antialias:a,premultipliedAlpha:c,preserveDrawingBuffer:l,powerPreference:f,failIfMajorPerformanceCaveat:d};if("setAttribute"in e&&e.setAttribute("data-engine","three.js r169"),e.addEventListener("webglcontextlost",$,!1),e.addEventListener("webglcontextrestored",pt,!1),e.addEventListener("webglcontextcreationerror",vt,!1),I===null){let U="webgl2";if(I=me(U,w),I===null)throw me(U)?new Error("Error creating WebGL context with your selected attributes."):new Error("Error creating WebGL context.")}}catch(w){throw console.error("THREE.WebGLRenderer: "+w.message),w}let j,ct,ut,Rt,z,M,v,P,H,K,k,nt,Q,X,ht,tt,lt,At,rt,ot,Lt,wt,Ft,L;function ft(){j=new V0(I),j.init(),wt=new yv(I,j),ct=new O0(I,j,t,wt),ut=new xv(I),ct.reverseDepthBuffer&&ut.buffers.depth.setReversed(!0),Rt=new X0(I),z=new sv,M=new _v(I,j,ut,z,ct,wt,Rt),v=new B0(y),P=new H0(y),H=new Qp(I),Ft=new U0(I,H),K=new G0(I,H,Rt,Ft),k=new Y0(I,K,H,Rt),rt=new q0(I,ct,M),tt=new F0(z),nt=new iv(y,v,P,j,ct,Ft,tt),Q=new Av(y,z),X=new ov,ht=new dv(j),At=new D0(y,v,P,ut,k,h,c),lt=new mv(y,k,ct),L=new Ev(I,Rt,ct,ut),ot=new N0(I,j,Rt),Lt=new W0(I,j,Rt),Rt.programs=nt.programs,y.capabilities=ct,y.extensions=j,y.properties=z,y.renderLists=X,y.shadowMap=lt,y.state=ut,y.info=Rt}ft();let Y=new vl(y,I);this.xr=Y,this.getContext=function(){return I},this.getContextAttributes=function(){return I.getContextAttributes()},this.forceContextLoss=function(){let w=j.get("WEBGL_lose_context");w&&w.loseContext()},this.forceContextRestore=function(){let w=j.get("WEBGL_lose_context");w&&w.restoreContext()},this.getPixelRatio=function(){return it},this.setPixelRatio=function(w){w!==void 0&&(it=w,this.setSize(et,B,!1))},this.getSize=function(w){return w.set(et,B)},this.setSize=function(w,U,G=!0){if(Y.isPresenting){console.warn("THREE.WebGLRenderer: Can't change size while VR device is presenting.");return}et=w,B=U,e.width=Math.floor(w*it),e.height=Math.floor(U*it),G===!0&&(e.style.width=w+"px",e.style.height=U+"px"),this.setViewport(0,0,w,U)},this.getDrawingBufferSize=function(w){return w.set(et*it,B*it).floor()},this.setDrawingBufferSize=function(w,U,G){et=w,B=U,it=G,e.width=Math.floor(w*G),e.height=Math.floor(U*G),this.setViewport(0,0,w,U)},this.getCurrentViewport=function(w){return w.copy(g)},this.getViewport=function(w){return w.copy(_t)},this.setViewport=function(w,U,G,W){w.isVector4?_t.set(w.x,w.y,w.z,w.w):_t.set(w,U,G,W),ut.viewport(g.copy(_t).multiplyScalar(it).round())},this.getScissor=function(w){return w.copy(bt)},this.setScissor=function(w,U,G,W){w.isVector4?bt.set(w.x,w.y,w.z,w.w):bt.set(w,U,G,W),ut.scissor(S.copy(bt).multiplyScalar(it).round())},this.getScissorTest=function(){return Gt},this.setScissorTest=function(w){ut.setScissorTest(Gt=w)},this.setOpaqueSort=function(w){Z=w},this.setTransparentSort=function(w){gt=w},this.getClearColor=function(w){return w.copy(At.getClearColor())},this.setClearColor=function(){At.setClearColor.apply(At,arguments)},this.getClearAlpha=function(){return At.getClearAlpha()},this.setClearAlpha=function(){At.setClearAlpha.apply(At,arguments)},this.clear=function(w=!0,U=!0,G=!0){let W=0;if(w){let O=!1;if(E!==null){let at=E.texture.format;O=at===kl||at===zl||at===Bl}if(O){let at=E.texture.type,mt=at===Fn||at===Ti||at===ar||at===ys||at===Nl||at===Ol,St=At.getClearColor(),Et=At.getClearAlpha(),Dt=St.r,Ut=St.g,Tt=St.b;mt?(m[0]=Dt,m[1]=Ut,m[2]=Tt,m[3]=Et,I.clearBufferuiv(I.COLOR,0,m)):(x[0]=Dt,x[1]=Ut,x[2]=Tt,x[3]=Et,I.clearBufferiv(I.COLOR,0,x))}else W|=I.COLOR_BUFFER_BIT}U&&(W|=I.DEPTH_BUFFER_BIT,I.clearDepth(this.capabilities.reverseDepthBuffer?0:1)),G&&(W|=I.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),I.clear(W)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.dispose=function(){e.removeEventListener("webglcontextlost",$,!1),e.removeEventListener("webglcontextrestored",pt,!1),e.removeEventListener("webglcontextcreationerror",vt,!1),X.dispose(),ht.dispose(),z.dispose(),v.dispose(),P.dispose(),k.dispose(),Ft.dispose(),L.dispose(),nt.dispose(),Y.dispose(),Y.removeEventListener("sessionstart",Rh),Y.removeEventListener("sessionend",Ph),pi.stop()};function $(w){w.preventDefault(),console.log("THREE.WebGLRenderer: Context Lost."),A=!0}function pt(){console.log("THREE.WebGLRenderer: Context Restored."),A=!1;let w=Rt.autoReset,U=lt.enabled,G=lt.autoUpdate,W=lt.needsUpdate,O=lt.type;ft(),Rt.autoReset=w,lt.enabled=U,lt.autoUpdate=G,lt.needsUpdate=W,lt.type=O}function vt(w){console.error("THREE.WebGLRenderer: A WebGL context could not be created. Reason: ",w.statusMessage)}function Xt(w){let U=w.target;U.removeEventListener("dispose",Xt),ue(U)}function ue(w){Fe(w),z.remove(w)}function Fe(w){let U=z.get(w).programs;U!==void 0&&(U.forEach(function(G){nt.releaseProgram(G)}),w.isShaderMaterial&&nt.releaseShaderCache(w))}this.renderBufferDirect=function(w,U,G,W,O,at){U===null&&(U=kt);let mt=O.isMesh&&O.matrixWorld.determinant()<0,St=Sf(w,U,G,W,O);ut.setMaterial(W,mt);let Et=G.index,Dt=1;if(W.wireframe===!0){if(Et=K.getWireframeAttribute(G),Et===void 0)return;Dt=2}let Ut=G.drawRange,Tt=G.attributes.position,Qt=Ut.start*Dt,ne=(Ut.start+Ut.count)*Dt;at!==null&&(Qt=Math.max(Qt,at.start*Dt),ne=Math.min(ne,(at.start+at.count)*Dt)),Et!==null?(Qt=Math.max(Qt,0),ne=Math.min(ne,Et.count)):Tt!=null&&(Qt=Math.max(Qt,0),ne=Math.min(ne,Tt.count));let ae=ne-Qt;if(ae<0||ae===1/0)return;Ft.setup(O,W,St,G,Et);let Xe,jt=ot;if(Et!==null&&(Xe=H.get(Et),jt=Lt,jt.setIndex(Xe)),O.isMesh)W.wireframe===!0?(ut.setLineWidth(W.wireframeLinewidth*Vt()),jt.setMode(I.LINES)):jt.setMode(I.TRIANGLES);else if(O.isLine){let Ct=W.linewidth;Ct===void 0&&(Ct=1),ut.setLineWidth(Ct*Vt()),O.isLineSegments?jt.setMode(I.LINES):O.isLineLoop?jt.setMode(I.LINE_LOOP):jt.setMode(I.LINE_STRIP)}else O.isPoints?jt.setMode(I.POINTS):O.isSprite&&jt.setMode(I.TRIANGLES);if(O.isBatchedMesh)if(O._multiDrawInstances!==null)jt.renderMultiDrawInstances(O._multiDrawStarts,O._multiDrawCounts,O._multiDrawCount,O._multiDrawInstances);else if(j.get("WEBGL_multi_draw"))jt.renderMultiDraw(O._multiDrawStarts,O._multiDrawCounts,O._multiDrawCount);else{let Ct=O._multiDrawStarts,Ae=O._multiDrawCounts,Kt=O._multiDrawCount,cn=Et?H.get(Et).bytesPerElement:1,ji=z.get(W).currentProgram.getUniforms();for(let qe=0;qe<Kt;qe++)ji.setValue(I,"_gl_DrawID",qe),jt.render(Ct[qe]/cn,Ae[qe])}else if(O.isInstancedMesh)jt.renderInstances(Qt,ae,O.count);else if(G.isInstancedBufferGeometry){let Ct=G._maxInstanceCount!==void 0?G._maxInstanceCount:1/0,Ae=Math.min(G.instanceCount,Ct);jt.renderInstances(Qt,ae,Ae)}else jt.render(Qt,ae)};function qt(w,U,G){w.transparent===!0&&w.side===Nn&&w.forceSinglePass===!1?(w.side=ze,w.needsUpdate=!0,Fr(w,U,G),w.side=ei,w.needsUpdate=!0,Fr(w,U,G),w.side=Nn):Fr(w,U,G)}this.compile=function(w,U,G=null){G===null&&(G=w),u=ht.get(G),u.init(U),b.push(u),G.traverseVisible(function(O){O.isLight&&O.layers.test(U.layers)&&(u.pushLight(O),O.castShadow&&u.pushShadow(O))}),w!==G&&w.traverseVisible(function(O){O.isLight&&O.layers.test(U.layers)&&(u.pushLight(O),O.castShadow&&u.pushShadow(O))}),u.setupLights();let W=new Set;return w.traverse(function(O){if(!(O.isMesh||O.isPoints||O.isLine||O.isSprite))return;let at=O.material;if(at)if(Array.isArray(at))for(let mt=0;mt<at.length;mt++){let St=at[mt];qt(St,G,O),W.add(St)}else qt(at,G,O),W.add(at)}),b.pop(),u=null,W},this.compileAsync=function(w,U,G=null){let W=this.compile(w,U,G);return new Promise(O=>{function at(){if(W.forEach(function(mt){z.get(mt).currentProgram.isReady()&&W.delete(mt)}),W.size===0){O(w);return}setTimeout(at,10)}j.get("KHR_parallel_shader_compile")!==null?at():setTimeout(at,10)})};let Be=null;function Cn(w){Be&&Be(w)}function Rh(){pi.stop()}function Ph(){pi.start()}let pi=new id;pi.setAnimationLoop(Cn),typeof self<"u"&&pi.setContext(self),this.setAnimationLoop=function(w){Be=w,Y.setAnimationLoop(w),w===null?pi.stop():pi.start()},Y.addEventListener("sessionstart",Rh),Y.addEventListener("sessionend",Ph),this.render=function(w,U){if(U!==void 0&&U.isCamera!==!0){console.error("THREE.WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(A===!0)return;if(w.matrixWorldAutoUpdate===!0&&w.updateMatrixWorld(),U.parent===null&&U.matrixWorldAutoUpdate===!0&&U.updateMatrixWorld(),Y.enabled===!0&&Y.isPresenting===!0&&(Y.cameraAutoUpdate===!0&&Y.updateCamera(U),U=Y.getCamera()),w.isScene===!0&&w.onBeforeRender(y,w,U,E),u=ht.get(w,b.length),u.init(U),b.push(u),xt.multiplyMatrices(U.projectionMatrix,U.matrixWorldInverse),Wt.setFromProjectionMatrix(xt),st=this.localClippingEnabled,J=tt.init(this.clippingPlanes,st),_=X.get(w,p.length),_.init(),p.push(_),Y.enabled===!0&&Y.isPresenting===!0){let at=y.xr.getDepthSensingMesh();at!==null&&Fa(at,U,-1/0,y.sortObjects)}Fa(w,U,0,y.sortObjects),_.finish(),y.sortObjects===!0&&_.sort(Z,gt),Zt=Y.enabled===!1||Y.isPresenting===!1||Y.hasDepthSensing()===!1,Zt&&At.addToRenderList(_,w),this.info.render.frame++,J===!0&&tt.beginShadows();let G=u.state.shadowsArray;lt.render(G,w,U),J===!0&&tt.endShadows(),this.info.autoReset===!0&&this.info.reset();let W=_.opaque,O=_.transmissive;if(u.setupLights(),U.isArrayCamera){let at=U.cameras;if(O.length>0)for(let mt=0,St=at.length;mt<St;mt++){let Et=at[mt];Lh(W,O,w,Et)}Zt&&At.render(w);for(let mt=0,St=at.length;mt<St;mt++){let Et=at[mt];Ih(_,w,Et,Et.viewport)}}else O.length>0&&Lh(W,O,w,U),Zt&&At.render(w),Ih(_,w,U);E!==null&&(M.updateMultisampleRenderTarget(E),M.updateRenderTargetMipmap(E)),w.isScene===!0&&w.onAfterRender(y,w,U),Ft.resetDefaultState(),C=-1,N=null,b.pop(),b.length>0?(u=b[b.length-1],J===!0&&tt.setGlobalState(y.clippingPlanes,u.state.camera)):u=null,p.pop(),p.length>0?_=p[p.length-1]:_=null};function Fa(w,U,G,W){if(w.visible===!1)return;if(w.layers.test(U.layers)){if(w.isGroup)G=w.renderOrder;else if(w.isLOD)w.autoUpdate===!0&&w.update(U);else if(w.isLight)u.pushLight(w),w.castShadow&&u.pushShadow(w);else if(w.isSprite){if(!w.frustumCulled||Wt.intersectsSprite(w)){W&&It.setFromMatrixPosition(w.matrixWorld).applyMatrix4(xt);let mt=k.update(w),St=w.material;St.visible&&_.push(w,mt,St,G,It.z,null)}}else if((w.isMesh||w.isLine||w.isPoints)&&(!w.frustumCulled||Wt.intersectsObject(w))){let mt=k.update(w),St=w.material;if(W&&(w.boundingSphere!==void 0?(w.boundingSphere===null&&w.computeBoundingSphere(),It.copy(w.boundingSphere.center)):(mt.boundingSphere===null&&mt.computeBoundingSphere(),It.copy(mt.boundingSphere.center)),It.applyMatrix4(w.matrixWorld).applyMatrix4(xt)),Array.isArray(St)){let Et=mt.groups;for(let Dt=0,Ut=Et.length;Dt<Ut;Dt++){let Tt=Et[Dt],Qt=St[Tt.materialIndex];Qt&&Qt.visible&&_.push(w,mt,Qt,G,It.z,Tt)}}else St.visible&&_.push(w,mt,St,G,It.z,null)}}let at=w.children;for(let mt=0,St=at.length;mt<St;mt++)Fa(at[mt],U,G,W)}function Ih(w,U,G,W){let O=w.opaque,at=w.transmissive,mt=w.transparent;u.setupLightsView(G),J===!0&&tt.setGlobalState(y.clippingPlanes,G),W&&ut.viewport(g.copy(W)),O.length>0&&Or(O,U,G),at.length>0&&Or(at,U,G),mt.length>0&&Or(mt,U,G),ut.buffers.depth.setTest(!0),ut.buffers.depth.setMask(!0),ut.buffers.color.setMask(!0),ut.setPolygonOffset(!1)}function Lh(w,U,G,W){if((G.isScene===!0?G.overrideMaterial:null)!==null)return;u.state.transmissionRenderTarget[W.id]===void 0&&(u.state.transmissionRenderTarget[W.id]=new fe(1,1,{generateMipmaps:!0,type:j.has("EXT_color_buffer_half_float")||j.has("EXT_color_buffer_float")?Ve:Fn,minFilter:Ei,samples:4,stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:Jt.workingColorSpace}));let at=u.state.transmissionRenderTarget[W.id],mt=W.viewport||g;at.setSize(mt.z,mt.w);let St=y.getRenderTarget();y.setRenderTarget(at),y.getClearColor(V),q=y.getClearAlpha(),q<1&&y.setClearColor(16777215,.5),y.clear(),Zt&&At.render(G);let Et=y.toneMapping;y.toneMapping=ti;let Dt=W.viewport;if(W.viewport!==void 0&&(W.viewport=void 0),u.setupLightsView(W),J===!0&&tt.setGlobalState(y.clippingPlanes,W),Or(w,G,W),M.updateMultisampleRenderTarget(at),M.updateRenderTargetMipmap(at),j.has("WEBGL_multisampled_render_to_texture")===!1){let Ut=!1;for(let Tt=0,Qt=U.length;Tt<Qt;Tt++){let ne=U[Tt],ae=ne.object,Xe=ne.geometry,jt=ne.material,Ct=ne.group;if(jt.side===Nn&&ae.layers.test(W.layers)){let Ae=jt.side;jt.side=ze,jt.needsUpdate=!0,Dh(ae,G,W,Xe,jt,Ct),jt.side=Ae,jt.needsUpdate=!0,Ut=!0}}Ut===!0&&(M.updateMultisampleRenderTarget(at),M.updateRenderTargetMipmap(at))}y.setRenderTarget(St),y.setClearColor(V,q),Dt!==void 0&&(W.viewport=Dt),y.toneMapping=Et}function Or(w,U,G){let W=U.isScene===!0?U.overrideMaterial:null;for(let O=0,at=w.length;O<at;O++){let mt=w[O],St=mt.object,Et=mt.geometry,Dt=W===null?mt.material:W,Ut=mt.group;St.layers.test(G.layers)&&Dh(St,U,G,Et,Dt,Ut)}}function Dh(w,U,G,W,O,at){w.onBeforeRender(y,U,G,W,O,at),w.modelViewMatrix.multiplyMatrices(G.matrixWorldInverse,w.matrixWorld),w.normalMatrix.getNormalMatrix(w.modelViewMatrix),O.onBeforeRender(y,U,G,W,w,at),O.transparent===!0&&O.side===Nn&&O.forceSinglePass===!1?(O.side=ze,O.needsUpdate=!0,y.renderBufferDirect(G,U,W,O,w,at),O.side=ei,O.needsUpdate=!0,y.renderBufferDirect(G,U,W,O,w,at),O.side=Nn):y.renderBufferDirect(G,U,W,O,w,at),w.onAfterRender(y,U,G,W,O,at)}function Fr(w,U,G){U.isScene!==!0&&(U=kt);let W=z.get(w),O=u.state.lights,at=u.state.shadowsArray,mt=O.state.version,St=nt.getParameters(w,O.state,at,U,G),Et=nt.getProgramCacheKey(St),Dt=W.programs;W.environment=w.isMeshStandardMaterial?U.environment:null,W.fog=U.fog,W.envMap=(w.isMeshStandardMaterial?P:v).get(w.envMap||W.environment),W.envMapRotation=W.environment!==null&&w.envMap===null?U.environmentRotation:w.envMapRotation,Dt===void 0&&(w.addEventListener("dispose",Xt),Dt=new Map,W.programs=Dt);let Ut=Dt.get(Et);if(Ut!==void 0){if(W.currentProgram===Ut&&W.lightsStateVersion===mt)return Nh(w,St),Ut}else St.uniforms=nt.getUniforms(w),w.onBeforeCompile(St,y),Ut=nt.acquireProgram(St,Et),Dt.set(Et,Ut),W.uniforms=St.uniforms;let Tt=W.uniforms;return(!w.isShaderMaterial&&!w.isRawShaderMaterial||w.clipping===!0)&&(Tt.clippingPlanes=tt.uniform),Nh(w,St),W.needsLights=Af(w),W.lightsStateVersion=mt,W.needsLights&&(Tt.ambientLightColor.value=O.state.ambient,Tt.lightProbe.value=O.state.probe,Tt.directionalLights.value=O.state.directional,Tt.directionalLightShadows.value=O.state.directionalShadow,Tt.spotLights.value=O.state.spot,Tt.spotLightShadows.value=O.state.spotShadow,Tt.rectAreaLights.value=O.state.rectArea,Tt.ltc_1.value=O.state.rectAreaLTC1,Tt.ltc_2.value=O.state.rectAreaLTC2,Tt.pointLights.value=O.state.point,Tt.pointLightShadows.value=O.state.pointShadow,Tt.hemisphereLights.value=O.state.hemi,Tt.directionalShadowMap.value=O.state.directionalShadowMap,Tt.directionalShadowMatrix.value=O.state.directionalShadowMatrix,Tt.spotShadowMap.value=O.state.spotShadowMap,Tt.spotLightMatrix.value=O.state.spotLightMatrix,Tt.spotLightMap.value=O.state.spotLightMap,Tt.pointShadowMap.value=O.state.pointShadowMap,Tt.pointShadowMatrix.value=O.state.pointShadowMatrix),W.currentProgram=Ut,W.uniformsList=null,Ut}function Uh(w){if(w.uniformsList===null){let U=w.currentProgram.getUniforms();w.uniformsList=gs.seqWithValue(U.seq,w.uniforms)}return w.uniformsList}function Nh(w,U){let G=z.get(w);G.outputColorSpace=U.outputColorSpace,G.batching=U.batching,G.batchingColor=U.batchingColor,G.instancing=U.instancing,G.instancingColor=U.instancingColor,G.instancingMorph=U.instancingMorph,G.skinning=U.skinning,G.morphTargets=U.morphTargets,G.morphNormals=U.morphNormals,G.morphColors=U.morphColors,G.morphTargetsCount=U.morphTargetsCount,G.numClippingPlanes=U.numClippingPlanes,G.numIntersection=U.numClipIntersection,G.vertexAlphas=U.vertexAlphas,G.vertexTangents=U.vertexTangents,G.toneMapping=U.toneMapping}function Sf(w,U,G,W,O){U.isScene!==!0&&(U=kt),M.resetTextureUnits();let at=U.fog,mt=W.isMeshStandardMaterial?U.environment:null,St=E===null?y.outputColorSpace:E.isXRRenderTarget===!0?E.texture.colorSpace:si,Et=(W.isMeshStandardMaterial?P:v).get(W.envMap||mt),Dt=W.vertexColors===!0&&!!G.attributes.color&&G.attributes.color.itemSize===4,Ut=!!G.attributes.tangent&&(!!W.normalMap||W.anisotropy>0),Tt=!!G.morphAttributes.position,Qt=!!G.morphAttributes.normal,ne=!!G.morphAttributes.color,ae=ti;W.toneMapped&&(E===null||E.isXRRenderTarget===!0)&&(ae=y.toneMapping);let Xe=G.morphAttributes.position||G.morphAttributes.normal||G.morphAttributes.color,jt=Xe!==void 0?Xe.length:0,Ct=z.get(W),Ae=u.state.lights;if(J===!0&&(st===!0||w!==N)){let tn=w===N&&W.id===C;tt.setState(W,w,tn)}let Kt=!1;W.version===Ct.__version?(Ct.needsLights&&Ct.lightsStateVersion!==Ae.state.version||Ct.outputColorSpace!==St||O.isBatchedMesh&&Ct.batching===!1||!O.isBatchedMesh&&Ct.batching===!0||O.isBatchedMesh&&Ct.batchingColor===!0&&O.colorTexture===null||O.isBatchedMesh&&Ct.batchingColor===!1&&O.colorTexture!==null||O.isInstancedMesh&&Ct.instancing===!1||!O.isInstancedMesh&&Ct.instancing===!0||O.isSkinnedMesh&&Ct.skinning===!1||!O.isSkinnedMesh&&Ct.skinning===!0||O.isInstancedMesh&&Ct.instancingColor===!0&&O.instanceColor===null||O.isInstancedMesh&&Ct.instancingColor===!1&&O.instanceColor!==null||O.isInstancedMesh&&Ct.instancingMorph===!0&&O.morphTexture===null||O.isInstancedMesh&&Ct.instancingMorph===!1&&O.morphTexture!==null||Ct.envMap!==Et||W.fog===!0&&Ct.fog!==at||Ct.numClippingPlanes!==void 0&&(Ct.numClippingPlanes!==tt.numPlanes||Ct.numIntersection!==tt.numIntersection)||Ct.vertexAlphas!==Dt||Ct.vertexTangents!==Ut||Ct.morphTargets!==Tt||Ct.morphNormals!==Qt||Ct.morphColors!==ne||Ct.toneMapping!==ae||Ct.morphTargetsCount!==jt)&&(Kt=!0):(Kt=!0,Ct.__version=W.version);let cn=Ct.currentProgram;Kt===!0&&(cn=Fr(W,U,O));let ji=!1,qe=!1,Ba=!1,he=cn.getUniforms(),Xn=Ct.uniforms;if(ut.useProgram(cn.program)&&(ji=!0,qe=!0,Ba=!0),W.id!==C&&(C=W.id,qe=!0),ji||N!==w){ct.reverseDepthBuffer?(Mt.copy(w.projectionMatrix),Lp(Mt),Dp(Mt),he.setValue(I,"projectionMatrix",Mt)):he.setValue(I,"projectionMatrix",w.projectionMatrix),he.setValue(I,"viewMatrix",w.matrixWorldInverse);let tn=he.map.cameraPosition;tn!==void 0&&tn.setValue(I,Nt.setFromMatrixPosition(w.matrixWorld)),ct.logarithmicDepthBuffer&&he.setValue(I,"logDepthBufFC",2/(Math.log(w.far+1)/Math.LN2)),(W.isMeshPhongMaterial||W.isMeshToonMaterial||W.isMeshLambertMaterial||W.isMeshBasicMaterial||W.isMeshStandardMaterial||W.isShaderMaterial)&&he.setValue(I,"isOrthographic",w.isOrthographicCamera===!0),N!==w&&(N=w,qe=!0,Ba=!0)}if(O.isSkinnedMesh){he.setOptional(I,O,"bindMatrix"),he.setOptional(I,O,"bindMatrixInverse");let tn=O.skeleton;tn&&(tn.boneTexture===null&&tn.computeBoneTexture(),he.setValue(I,"boneTexture",tn.boneTexture,M))}O.isBatchedMesh&&(he.setOptional(I,O,"batchingTexture"),he.setValue(I,"batchingTexture",O._matricesTexture,M),he.setOptional(I,O,"batchingIdTexture"),he.setValue(I,"batchingIdTexture",O._indirectTexture,M),he.setOptional(I,O,"batchingColorTexture"),O._colorsTexture!==null&&he.setValue(I,"batchingColorTexture",O._colorsTexture,M));let za=G.morphAttributes;if((za.position!==void 0||za.normal!==void 0||za.color!==void 0)&&rt.update(O,G,cn),(qe||Ct.receiveShadow!==O.receiveShadow)&&(Ct.receiveShadow=O.receiveShadow,he.setValue(I,"receiveShadow",O.receiveShadow)),W.isMeshGouraudMaterial&&W.envMap!==null&&(Xn.envMap.value=Et,Xn.flipEnvMap.value=Et.isCubeTexture&&Et.isRenderTargetTexture===!1?-1:1),W.isMeshStandardMaterial&&W.envMap===null&&U.environment!==null&&(Xn.envMapIntensity.value=U.environmentIntensity),qe&&(he.setValue(I,"toneMappingExposure",y.toneMappingExposure),Ct.needsLights&&wf(Xn,Ba),at&&W.fog===!0&&Q.refreshFogUniforms(Xn,at),Q.refreshMaterialUniforms(Xn,W,it,B,u.state.transmissionRenderTarget[w.id]),gs.upload(I,Uh(Ct),Xn,M)),W.isShaderMaterial&&W.uniformsNeedUpdate===!0&&(gs.upload(I,Uh(Ct),Xn,M),W.uniformsNeedUpdate=!1),W.isSpriteMaterial&&he.setValue(I,"center",O.center),he.setValue(I,"modelViewMatrix",O.modelViewMatrix),he.setValue(I,"normalMatrix",O.normalMatrix),he.setValue(I,"modelMatrix",O.matrixWorld),W.isShaderMaterial||W.isRawShaderMaterial){let tn=W.uniformsGroups;for(let ka=0,Ef=tn.length;ka<Ef;ka++){let Oh=tn[ka];L.update(Oh,cn),L.bind(Oh,cn)}}return cn}function wf(w,U){w.ambientLightColor.needsUpdate=U,w.lightProbe.needsUpdate=U,w.directionalLights.needsUpdate=U,w.directionalLightShadows.needsUpdate=U,w.pointLights.needsUpdate=U,w.pointLightShadows.needsUpdate=U,w.spotLights.needsUpdate=U,w.spotLightShadows.needsUpdate=U,w.rectAreaLights.needsUpdate=U,w.hemisphereLights.needsUpdate=U}function Af(w){return w.isMeshLambertMaterial||w.isMeshToonMaterial||w.isMeshPhongMaterial||w.isMeshStandardMaterial||w.isShadowMaterial||w.isShaderMaterial&&w.lights===!0}this.getActiveCubeFace=function(){return R},this.getActiveMipmapLevel=function(){return T},this.getRenderTarget=function(){return E},this.setRenderTargetTextures=function(w,U,G){z.get(w.texture).__webglTexture=U,z.get(w.depthTexture).__webglTexture=G;let W=z.get(w);W.__hasExternalTextures=!0,W.__autoAllocateDepthBuffer=G===void 0,W.__autoAllocateDepthBuffer||j.has("WEBGL_multisampled_render_to_texture")===!0&&(console.warn("THREE.WebGLRenderer: Render-to-texture extension was disabled because an external texture was provided"),W.__useRenderToTexture=!1)},this.setRenderTargetFramebuffer=function(w,U){let G=z.get(w);G.__webglFramebuffer=U,G.__useDefaultFramebuffer=U===void 0},this.setRenderTarget=function(w,U=0,G=0){E=w,R=U,T=G;let W=!0,O=null,at=!1,mt=!1;if(w){let Et=z.get(w);if(Et.__useDefaultFramebuffer!==void 0)ut.bindFramebuffer(I.FRAMEBUFFER,null),W=!1;else if(Et.__webglFramebuffer===void 0)M.setupRenderTarget(w);else if(Et.__hasExternalTextures)M.rebindTextures(w,z.get(w.texture).__webglTexture,z.get(w.depthTexture).__webglTexture);else if(w.depthBuffer){let Tt=w.depthTexture;if(Et.__boundDepthTexture!==Tt){if(Tt!==null&&z.has(Tt)&&(w.width!==Tt.image.width||w.height!==Tt.image.height))throw new Error("WebGLRenderTarget: Attached DepthTexture is initialized to the incorrect size.");M.setupDepthRenderbuffer(w)}}let Dt=w.texture;(Dt.isData3DTexture||Dt.isDataArrayTexture||Dt.isCompressedArrayTexture)&&(mt=!0);let Ut=z.get(w).__webglFramebuffer;w.isWebGLCubeRenderTarget?(Array.isArray(Ut[U])?O=Ut[U][G]:O=Ut[U],at=!0):w.samples>0&&M.useMultisampledRTT(w)===!1?O=z.get(w).__webglMultisampledFramebuffer:Array.isArray(Ut)?O=Ut[G]:O=Ut,g.copy(w.viewport),S.copy(w.scissor),F=w.scissorTest}else g.copy(_t).multiplyScalar(it).floor(),S.copy(bt).multiplyScalar(it).floor(),F=Gt;if(ut.bindFramebuffer(I.FRAMEBUFFER,O)&&W&&ut.drawBuffers(w,O),ut.viewport(g),ut.scissor(S),ut.setScissorTest(F),at){let Et=z.get(w.texture);I.framebufferTexture2D(I.FRAMEBUFFER,I.COLOR_ATTACHMENT0,I.TEXTURE_CUBE_MAP_POSITIVE_X+U,Et.__webglTexture,G)}else if(mt){let Et=z.get(w.texture),Dt=U||0;I.framebufferTextureLayer(I.FRAMEBUFFER,I.COLOR_ATTACHMENT0,Et.__webglTexture,G||0,Dt)}C=-1},this.readRenderTargetPixels=function(w,U,G,W,O,at,mt){if(!(w&&w.isWebGLRenderTarget)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let St=z.get(w).__webglFramebuffer;if(w.isWebGLCubeRenderTarget&&mt!==void 0&&(St=St[mt]),St){ut.bindFramebuffer(I.FRAMEBUFFER,St);try{let Et=w.texture,Dt=Et.format,Ut=Et.type;if(!ct.textureFormatReadable(Dt)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!ct.textureTypeReadable(Ut)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}U>=0&&U<=w.width-W&&G>=0&&G<=w.height-O&&I.readPixels(U,G,W,O,wt.convert(Dt),wt.convert(Ut),at)}finally{let Et=E!==null?z.get(E).__webglFramebuffer:null;ut.bindFramebuffer(I.FRAMEBUFFER,Et)}}},this.readRenderTargetPixelsAsync=async function(w,U,G,W,O,at,mt){if(!(w&&w.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let St=z.get(w).__webglFramebuffer;if(w.isWebGLCubeRenderTarget&&mt!==void 0&&(St=St[mt]),St){let Et=w.texture,Dt=Et.format,Ut=Et.type;if(!ct.textureFormatReadable(Dt))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!ct.textureTypeReadable(Ut))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");if(U>=0&&U<=w.width-W&&G>=0&&G<=w.height-O){ut.bindFramebuffer(I.FRAMEBUFFER,St);let Tt=I.createBuffer();I.bindBuffer(I.PIXEL_PACK_BUFFER,Tt),I.bufferData(I.PIXEL_PACK_BUFFER,at.byteLength,I.STREAM_READ),I.readPixels(U,G,W,O,wt.convert(Dt),wt.convert(Ut),0);let Qt=E!==null?z.get(E).__webglFramebuffer:null;ut.bindFramebuffer(I.FRAMEBUFFER,Qt);let ne=I.fenceSync(I.SYNC_GPU_COMMANDS_COMPLETE,0);return I.flush(),await Ip(I,ne,4),I.bindBuffer(I.PIXEL_PACK_BUFFER,Tt),I.getBufferSubData(I.PIXEL_PACK_BUFFER,0,at),I.deleteBuffer(Tt),I.deleteSync(ne),at}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")}},this.copyFramebufferToTexture=function(w,U=null,G=0){w.isTexture!==!0&&(fo("WebGLRenderer: copyFramebufferToTexture function signature has changed."),U=arguments[0]||null,w=arguments[1]);let W=Math.pow(2,-G),O=Math.floor(w.image.width*W),at=Math.floor(w.image.height*W),mt=U!==null?U.x:0,St=U!==null?U.y:0;M.setTexture2D(w,0),I.copyTexSubImage2D(I.TEXTURE_2D,G,0,0,mt,St,O,at),ut.unbindTexture()},this.copyTextureToTexture=function(w,U,G=null,W=null,O=0){w.isTexture!==!0&&(fo("WebGLRenderer: copyTextureToTexture function signature has changed."),W=arguments[0]||null,w=arguments[1],U=arguments[2],O=arguments[3]||0,G=null);let at,mt,St,Et,Dt,Ut;G!==null?(at=G.max.x-G.min.x,mt=G.max.y-G.min.y,St=G.min.x,Et=G.min.y):(at=w.image.width,mt=w.image.height,St=0,Et=0),W!==null?(Dt=W.x,Ut=W.y):(Dt=0,Ut=0);let Tt=wt.convert(U.format),Qt=wt.convert(U.type);M.setTexture2D(U,0),I.pixelStorei(I.UNPACK_FLIP_Y_WEBGL,U.flipY),I.pixelStorei(I.UNPACK_PREMULTIPLY_ALPHA_WEBGL,U.premultiplyAlpha),I.pixelStorei(I.UNPACK_ALIGNMENT,U.unpackAlignment);let ne=I.getParameter(I.UNPACK_ROW_LENGTH),ae=I.getParameter(I.UNPACK_IMAGE_HEIGHT),Xe=I.getParameter(I.UNPACK_SKIP_PIXELS),jt=I.getParameter(I.UNPACK_SKIP_ROWS),Ct=I.getParameter(I.UNPACK_SKIP_IMAGES),Ae=w.isCompressedTexture?w.mipmaps[O]:w.image;I.pixelStorei(I.UNPACK_ROW_LENGTH,Ae.width),I.pixelStorei(I.UNPACK_IMAGE_HEIGHT,Ae.height),I.pixelStorei(I.UNPACK_SKIP_PIXELS,St),I.pixelStorei(I.UNPACK_SKIP_ROWS,Et),w.isDataTexture?I.texSubImage2D(I.TEXTURE_2D,O,Dt,Ut,at,mt,Tt,Qt,Ae.data):w.isCompressedTexture?I.compressedTexSubImage2D(I.TEXTURE_2D,O,Dt,Ut,Ae.width,Ae.height,Tt,Ae.data):I.texSubImage2D(I.TEXTURE_2D,O,Dt,Ut,at,mt,Tt,Qt,Ae),I.pixelStorei(I.UNPACK_ROW_LENGTH,ne),I.pixelStorei(I.UNPACK_IMAGE_HEIGHT,ae),I.pixelStorei(I.UNPACK_SKIP_PIXELS,Xe),I.pixelStorei(I.UNPACK_SKIP_ROWS,jt),I.pixelStorei(I.UNPACK_SKIP_IMAGES,Ct),O===0&&U.generateMipmaps&&I.generateMipmap(I.TEXTURE_2D),ut.unbindTexture()},this.copyTextureToTexture3D=function(w,U,G=null,W=null,O=0){w.isTexture!==!0&&(fo("WebGLRenderer: copyTextureToTexture3D function signature has changed."),G=arguments[0]||null,W=arguments[1]||null,w=arguments[2],U=arguments[3],O=arguments[4]||0);let at,mt,St,Et,Dt,Ut,Tt,Qt,ne,ae=w.isCompressedTexture?w.mipmaps[O]:w.image;G!==null?(at=G.max.x-G.min.x,mt=G.max.y-G.min.y,St=G.max.z-G.min.z,Et=G.min.x,Dt=G.min.y,Ut=G.min.z):(at=ae.width,mt=ae.height,St=ae.depth,Et=0,Dt=0,Ut=0),W!==null?(Tt=W.x,Qt=W.y,ne=W.z):(Tt=0,Qt=0,ne=0);let Xe=wt.convert(U.format),jt=wt.convert(U.type),Ct;if(U.isData3DTexture)M.setTexture3D(U,0),Ct=I.TEXTURE_3D;else if(U.isDataArrayTexture||U.isCompressedArrayTexture)M.setTexture2DArray(U,0),Ct=I.TEXTURE_2D_ARRAY;else{console.warn("THREE.WebGLRenderer.copyTextureToTexture3D: only supports THREE.DataTexture3D and THREE.DataTexture2DArray.");return}I.pixelStorei(I.UNPACK_FLIP_Y_WEBGL,U.flipY),I.pixelStorei(I.UNPACK_PREMULTIPLY_ALPHA_WEBGL,U.premultiplyAlpha),I.pixelStorei(I.UNPACK_ALIGNMENT,U.unpackAlignment);let Ae=I.getParameter(I.UNPACK_ROW_LENGTH),Kt=I.getParameter(I.UNPACK_IMAGE_HEIGHT),cn=I.getParameter(I.UNPACK_SKIP_PIXELS),ji=I.getParameter(I.UNPACK_SKIP_ROWS),qe=I.getParameter(I.UNPACK_SKIP_IMAGES);I.pixelStorei(I.UNPACK_ROW_LENGTH,ae.width),I.pixelStorei(I.UNPACK_IMAGE_HEIGHT,ae.height),I.pixelStorei(I.UNPACK_SKIP_PIXELS,Et),I.pixelStorei(I.UNPACK_SKIP_ROWS,Dt),I.pixelStorei(I.UNPACK_SKIP_IMAGES,Ut),w.isDataTexture||w.isData3DTexture?I.texSubImage3D(Ct,O,Tt,Qt,ne,at,mt,St,Xe,jt,ae.data):U.isCompressedArrayTexture?I.compressedTexSubImage3D(Ct,O,Tt,Qt,ne,at,mt,St,Xe,ae.data):I.texSubImage3D(Ct,O,Tt,Qt,ne,at,mt,St,Xe,jt,ae),I.pixelStorei(I.UNPACK_ROW_LENGTH,Ae),I.pixelStorei(I.UNPACK_IMAGE_HEIGHT,Kt),I.pixelStorei(I.UNPACK_SKIP_PIXELS,cn),I.pixelStorei(I.UNPACK_SKIP_ROWS,ji),I.pixelStorei(I.UNPACK_SKIP_IMAGES,qe),O===0&&U.generateMipmaps&&I.generateMipmap(Ct),ut.unbindTexture()},this.initRenderTarget=function(w){z.get(w).__webglFramebuffer===void 0&&M.setupRenderTarget(w)},this.initTexture=function(w){w.isCubeTexture?M.setTextureCube(w,0):w.isData3DTexture?M.setTexture3D(w,0):w.isDataArrayTexture||w.isCompressedArrayTexture?M.setTexture2DArray(w,0):M.setTexture2D(w,0),ut.unbindTexture()},this.resetState=function(){R=0,T=0,E=null,ut.reset(),Ft.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return On}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(t){this._outputColorSpace=t;let e=this.getContext();e.drawingBufferColorSpace=t===Hl?"display-p3":"srgb",e.unpackColorSpace=Jt.workingColorSpace===Vo?"display-p3":"srgb"}},Io=class n{constructor(t,e=25e-5){this.isFogExp2=!0,this.name="",this.color=new Ot(t),this.density=e}clone(){return new n(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Lo=class extends He{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new nn,this.environmentIntensity=1,this.environmentRotation=new nn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(t,e){return super.copy(t,e),t.background!==null&&(this.background=t.background.clone()),t.environment!==null&&(this.environment=t.environment.clone()),t.fog!==null&&(this.fog=t.fog.clone()),this.backgroundBlurriness=t.backgroundBlurriness,this.backgroundIntensity=t.backgroundIntensity,this.backgroundRotation.copy(t.backgroundRotation),this.environmentIntensity=t.environmentIntensity,this.environmentRotation.copy(t.environmentRotation),t.overrideMaterial!==null&&(this.overrideMaterial=t.overrideMaterial.clone()),this.matrixAutoUpdate=t.matrixAutoUpdate,this}toJSON(t){let e=super.toJSON(t);return this.fog!==null&&(e.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(e.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(e.object.backgroundIntensity=this.backgroundIntensity),e.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(e.object.environmentIntensity=this.environmentIntensity),e.object.environmentRotation=this.environmentRotation.toArray(),e}};var _l=class extends Pe{constructor(t=null,e=1,i=1,s,r,o,a,c,l=Ee,f=Ee,d,h){super(null,o,a,c,l,f,s,r,d,h),this.isDataTexture=!0,this.image={data:t,width:e,height:i},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var ve=class extends Ke{constructor(t,e,i,s=1){super(t,e,i),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=s}copy(t){return super.copy(t),this.meshPerAttribute=t.meshPerAttribute,this}toJSON(){let t=super.toJSON();return t.meshPerAttribute=this.meshPerAttribute,t.isInstancedBufferAttribute=!0,t}},hs=new $t,Du=new $t,ro=[],Uu=new zn,Tv=new $t,er=new Oe,nr=new Ci,ii=class extends Oe{constructor(t,e,i){super(t,e),this.isInstancedMesh=!0,this.instanceMatrix=new ve(new Float32Array(i*16),16),this.instanceColor=null,this.morphTexture=null,this.count=i,this.boundingBox=null,this.boundingSphere=null;for(let s=0;s<i;s++)this.setMatrixAt(s,Tv)}computeBoundingBox(){let t=this.geometry,e=this.count;this.boundingBox===null&&(this.boundingBox=new zn),t.boundingBox===null&&t.computeBoundingBox(),this.boundingBox.makeEmpty();for(let i=0;i<e;i++)this.getMatrixAt(i,hs),Uu.copy(t.boundingBox).applyMatrix4(hs),this.boundingBox.union(Uu)}computeBoundingSphere(){let t=this.geometry,e=this.count;this.boundingSphere===null&&(this.boundingSphere=new Ci),t.boundingSphere===null&&t.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let i=0;i<e;i++)this.getMatrixAt(i,hs),nr.copy(t.boundingSphere).applyMatrix4(hs),this.boundingSphere.union(nr)}copy(t,e){return super.copy(t,e),this.instanceMatrix.copy(t.instanceMatrix),t.morphTexture!==null&&(this.morphTexture=t.morphTexture.clone()),t.instanceColor!==null&&(this.instanceColor=t.instanceColor.clone()),this.count=t.count,t.boundingBox!==null&&(this.boundingBox=t.boundingBox.clone()),t.boundingSphere!==null&&(this.boundingSphere=t.boundingSphere.clone()),this}getColorAt(t,e){e.fromArray(this.instanceColor.array,t*3)}getMatrixAt(t,e){e.fromArray(this.instanceMatrix.array,t*16)}getMorphAt(t,e){let i=e.morphTargetInfluences,s=this.morphTexture.source.data.data,r=i.length+1,o=t*r+1;for(let a=0;a<i.length;a++)i[a]=s[o+a]}raycast(t,e){let i=this.matrixWorld,s=this.count;if(er.geometry=this.geometry,er.material=this.material,er.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),nr.copy(this.boundingSphere),nr.applyMatrix4(i),t.ray.intersectsSphere(nr)!==!1))for(let r=0;r<s;r++){this.getMatrixAt(r,hs),Du.multiplyMatrices(i,hs),er.matrixWorld=Du,er.raycast(t,ro);for(let o=0,a=ro.length;o<a;o++){let c=ro[o];c.instanceId=r,c.object=this,e.push(c)}ro.length=0}}setColorAt(t,e){this.instanceColor===null&&(this.instanceColor=new ve(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),e.toArray(this.instanceColor.array,t*3)}setMatrixAt(t,e){e.toArray(this.instanceMatrix.array,t*16)}setMorphAt(t,e){let i=e.morphTargetInfluences,s=i.length+1;this.morphTexture===null&&(this.morphTexture=new _l(new Float32Array(s*this.count),s,this.count,Fl,bn));let r=this.morphTexture.source.data.data,o=0;for(let l=0;l<i.length;l++)o+=i[l];let a=this.geometry.morphTargetsRelative?1:1-o,c=s*t;r[c]=a,r.set(i,c+1)}updateMorphTargets(){}dispose(){return this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null),this}};var dr=class n extends pn{constructor(t=1,e=1,i=1,s=32,r=1,o=!1,a=0,c=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:t,radiusBottom:e,height:i,radialSegments:s,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:c};let l=this;s=Math.floor(s),r=Math.floor(r);let f=[],d=[],h=[],m=[],x=0,_=[],u=i/2,p=0;b(),o===!1&&(t>0&&y(!0),e>0&&y(!1)),this.setIndex(f),this.setAttribute("position",new be(d,3)),this.setAttribute("normal",new be(h,3)),this.setAttribute("uv",new be(m,2));function b(){let A=new D,R=new D,T=0,E=(e-t)/i;for(let C=0;C<=r;C++){let N=[],g=C/r,S=g*(e-t)+t;for(let F=0;F<=s;F++){let V=F/s,q=V*c+a,et=Math.sin(q),B=Math.cos(q);R.x=S*et,R.y=-g*i+u,R.z=S*B,d.push(R.x,R.y,R.z),A.set(et,E,B).normalize(),h.push(A.x,A.y,A.z),m.push(V,1-g),N.push(x++)}_.push(N)}for(let C=0;C<s;C++)for(let N=0;N<r;N++){let g=_[N][C],S=_[N+1][C],F=_[N+1][C+1],V=_[N][C+1];t>0&&(f.push(g,S,V),T+=3),e>0&&(f.push(S,F,V),T+=3)}l.addGroup(p,T,0),p+=T}function y(A){let R=x,T=new yt,E=new D,C=0,N=A===!0?t:e,g=A===!0?1:-1;for(let F=1;F<=s;F++)d.push(0,u*g,0),h.push(0,g,0),m.push(.5,.5),x++;let S=x;for(let F=0;F<=s;F++){let q=F/s*c+a,et=Math.cos(q),B=Math.sin(q);E.x=N*B,E.y=u*g,E.z=N*et,d.push(E.x,E.y,E.z),h.push(0,g,0),T.x=et*.5+.5,T.y=B*.5*g+.5,m.push(T.x,T.y),x++}for(let F=0;F<s;F++){let V=R+F,q=S+F;A===!0?f.push(q,q+1,V):f.push(q+1,q,V),C+=3}l.addGroup(p,C,A===!0?1:2),p+=C}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.radiusTop,t.radiusBottom,t.height,t.radialSegments,t.heightSegments,t.openEnded,t.thetaStart,t.thetaLength)}};var yl=class n extends pn{constructor(t=[],e=[],i=1,s=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:t,indices:e,radius:i,detail:s};let r=[],o=[];a(s),l(i),f(),this.setAttribute("position",new be(r,3)),this.setAttribute("normal",new be(r.slice(),3)),this.setAttribute("uv",new be(o,2)),s===0?this.computeVertexNormals():this.normalizeNormals();function a(b){let y=new D,A=new D,R=new D;for(let T=0;T<e.length;T+=3)m(e[T+0],y),m(e[T+1],A),m(e[T+2],R),c(y,A,R,b)}function c(b,y,A,R){let T=R+1,E=[];for(let C=0;C<=T;C++){E[C]=[];let N=b.clone().lerp(A,C/T),g=y.clone().lerp(A,C/T),S=T-C;for(let F=0;F<=S;F++)F===0&&C===T?E[C][F]=N:E[C][F]=N.clone().lerp(g,F/S)}for(let C=0;C<T;C++)for(let N=0;N<2*(T-C)-1;N++){let g=Math.floor(N/2);N%2===0?(h(E[C][g+1]),h(E[C+1][g]),h(E[C][g])):(h(E[C][g+1]),h(E[C+1][g+1]),h(E[C+1][g]))}}function l(b){let y=new D;for(let A=0;A<r.length;A+=3)y.x=r[A+0],y.y=r[A+1],y.z=r[A+2],y.normalize().multiplyScalar(b),r[A+0]=y.x,r[A+1]=y.y,r[A+2]=y.z}function f(){let b=new D;for(let y=0;y<r.length;y+=3){b.x=r[y+0],b.y=r[y+1],b.z=r[y+2];let A=u(b)/2/Math.PI+.5,R=p(b)/Math.PI+.5;o.push(A,1-R)}x(),d()}function d(){for(let b=0;b<o.length;b+=6){let y=o[b+0],A=o[b+2],R=o[b+4],T=Math.max(y,A,R),E=Math.min(y,A,R);T>.9&&E<.1&&(y<.2&&(o[b+0]+=1),A<.2&&(o[b+2]+=1),R<.2&&(o[b+4]+=1))}}function h(b){r.push(b.x,b.y,b.z)}function m(b,y){let A=b*3;y.x=t[A+0],y.y=t[A+1],y.z=t[A+2]}function x(){let b=new D,y=new D,A=new D,R=new D,T=new yt,E=new yt,C=new yt;for(let N=0,g=0;N<r.length;N+=9,g+=6){b.set(r[N+0],r[N+1],r[N+2]),y.set(r[N+3],r[N+4],r[N+5]),A.set(r[N+6],r[N+7],r[N+8]),T.set(o[g+0],o[g+1]),E.set(o[g+2],o[g+3]),C.set(o[g+4],o[g+5]),R.copy(b).add(y).add(A).divideScalar(3);let S=u(R);_(T,g+0,b,S),_(E,g+2,y,S),_(C,g+4,A,S)}}function _(b,y,A,R){R<0&&b.x===1&&(o[y]=b.x-1),A.x===0&&A.z===0&&(o[y]=R/2/Math.PI+.5)}function u(b){return Math.atan2(b.z,-b.x)}function p(b){return Math.atan2(-b.y,Math.sqrt(b.x*b.x+b.z*b.z))}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.vertices,t.indices,t.radius,t.details)}};var Do=class n extends yl{constructor(t=1,e=0){let i=(1+Math.sqrt(5))/2,s=[-1,i,0,1,i,0,-1,-i,0,1,-i,0,0,-1,i,0,1,i,0,-1,-i,0,1,-i,i,0,-1,i,0,1,-i,0,-1,-i,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(s,r,t,e),this.type="IcosahedronGeometry",this.parameters={radius:t,detail:e}}static fromJSON(t){return new n(t.radius,t.detail)}};var Uo=class extends Ri{constructor(t){super(),this.isMeshStandardMaterial=!0,this.defines={STANDARD:""},this.type="MeshStandardMaterial",this.color=new Ot(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ot(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Qu,this.normalScale=new yt(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new nn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(t)}copy(t){return super.copy(t),this.defines={STANDARD:""},this.color.copy(t.color),this.roughness=t.roughness,this.metalness=t.metalness,this.map=t.map,this.lightMap=t.lightMap,this.lightMapIntensity=t.lightMapIntensity,this.aoMap=t.aoMap,this.aoMapIntensity=t.aoMapIntensity,this.emissive.copy(t.emissive),this.emissiveMap=t.emissiveMap,this.emissiveIntensity=t.emissiveIntensity,this.bumpMap=t.bumpMap,this.bumpScale=t.bumpScale,this.normalMap=t.normalMap,this.normalMapType=t.normalMapType,this.normalScale.copy(t.normalScale),this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this.roughnessMap=t.roughnessMap,this.metalnessMap=t.metalnessMap,this.alphaMap=t.alphaMap,this.envMap=t.envMap,this.envMapRotation.copy(t.envMapRotation),this.envMapIntensity=t.envMapIntensity,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.wireframeLinecap=t.wireframeLinecap,this.wireframeLinejoin=t.wireframeLinejoin,this.flatShading=t.flatShading,this.fog=t.fog,this}};function oo(n,t,e){return!n||!e&&n.constructor===t?n:typeof t.BYTES_PER_ELEMENT=="number"?new t(n):Array.prototype.slice.call(n)}function Cv(n){return ArrayBuffer.isView(n)&&!(n instanceof DataView)}var As=class{constructor(t,e,i,s){this.parameterPositions=t,this._cachedIndex=0,this.resultBuffer=s!==void 0?s:new e.constructor(i),this.sampleValues=e,this.valueSize=i,this.settings=null,this.DefaultSettings_={}}evaluate(t){let e=this.parameterPositions,i=this._cachedIndex,s=e[i],r=e[i-1];n:{t:{let o;e:{i:if(!(t<s)){for(let a=i+2;;){if(s===void 0){if(t<r)break i;return i=e.length,this._cachedIndex=i,this.copySampleValue_(i-1)}if(i===a)break;if(r=s,s=e[++i],t<s)break t}o=e.length;break e}if(!(t>=r)){let a=e[1];t<a&&(i=2,r=a);for(let c=i-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===c)break;if(s=r,r=e[--i-1],t>=r)break t}o=i,i=0;break e}break n}for(;i<o;){let a=i+o>>>1;t<e[a]?o=a:i=a+1}if(s=e[i],r=e[i-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(s===void 0)return i=e.length,this._cachedIndex=i,this.copySampleValue_(i-1)}this._cachedIndex=i,this.intervalChanged_(i,r,s)}return this.interpolate_(i,r,t,s)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(t){let e=this.resultBuffer,i=this.sampleValues,s=this.valueSize,r=t*s;for(let o=0;o!==s;++o)e[o]=i[r+o];return e}interpolate_(){throw new Error("call to abstract method")}intervalChanged_(){}},Ml=class extends As{constructor(t,e,i,s){super(t,e,i,s),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:kh,endingEnd:kh}}intervalChanged_(t,e,i){let s=this.parameterPositions,r=t-2,o=t+1,a=s[r],c=s[o];if(a===void 0)switch(this.getSettings_().endingStart){case Hh:r=t,a=2*e-i;break;case Vh:r=s.length-2,a=e+s[r]-s[r+1];break;default:r=t,a=i}if(c===void 0)switch(this.getSettings_().endingEnd){case Hh:o=t,c=2*i-e;break;case Vh:o=1,c=i+s[1]-s[0];break;default:o=t-1,c=e}let l=(i-e)*.5,f=this.valueSize;this._weightPrev=l/(e-a),this._weightNext=l/(c-i),this._offsetPrev=r*f,this._offsetNext=o*f}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,c=t*a,l=c-a,f=this._offsetPrev,d=this._offsetNext,h=this._weightPrev,m=this._weightNext,x=(i-e)/(s-e),_=x*x,u=_*x,p=-h*u+2*h*_-h*x,b=(1+h)*u+(-1.5-2*h)*_+(-.5+h)*x+1,y=(-1-m)*u+(1.5+m)*_+.5*x,A=m*u-m*_;for(let R=0;R!==a;++R)r[R]=p*o[f+R]+b*o[l+R]+y*o[c+R]+A*o[d+R];return r}},bl=class extends As{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,c=t*a,l=c-a,f=(i-e)/(s-e),d=1-f;for(let h=0;h!==a;++h)r[h]=o[l+h]*d+o[c+h]*f;return r}},Sl=class extends As{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t){return this.copySampleValue_(t-1)}},mn=class{constructor(t,e,i,s){if(t===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(e===void 0||e.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+t);this.name=t,this.times=oo(e,this.TimeBufferType),this.values=oo(i,this.ValueBufferType),this.setInterpolation(s||this.DefaultInterpolation)}static toJSON(t){let e=t.constructor,i;if(e.toJSON!==this.toJSON)i=e.toJSON(t);else{i={name:t.name,times:oo(t.times,Array),values:oo(t.values,Array)};let s=t.getInterpolation();s!==t.DefaultInterpolation&&(i.interpolation=s)}return i.type=t.ValueTypeName,i}InterpolantFactoryMethodDiscrete(t){return new Sl(this.times,this.values,this.getValueSize(),t)}InterpolantFactoryMethodLinear(t){return new bl(this.times,this.values,this.getValueSize(),t)}InterpolantFactoryMethodSmooth(t){return new Ml(this.times,this.values,this.getValueSize(),t)}setInterpolation(t){let e;switch(t){case po:e=this.InterpolantFactoryMethodDiscrete;break;case nl:e=this.InterpolantFactoryMethodLinear;break;case Va:e=this.InterpolantFactoryMethodSmooth;break}if(e===void 0){let i="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(t!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(i);return console.warn("THREE.KeyframeTrack:",i),this}return this.createInterpolant=e,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return po;case this.InterpolantFactoryMethodLinear:return nl;case this.InterpolantFactoryMethodSmooth:return Va}}getValueSize(){return this.values.length/this.times.length}shift(t){if(t!==0){let e=this.times;for(let i=0,s=e.length;i!==s;++i)e[i]+=t}return this}scale(t){if(t!==1){let e=this.times;for(let i=0,s=e.length;i!==s;++i)e[i]*=t}return this}trim(t,e){let i=this.times,s=i.length,r=0,o=s-1;for(;r!==s&&i[r]<t;)++r;for(;o!==-1&&i[o]>e;)--o;if(++o,r!==0||o!==s){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=i.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let t=!0,e=this.getValueSize();e-Math.floor(e)!==0&&(console.error("THREE.KeyframeTrack: Invalid value size in track.",this),t=!1);let i=this.times,s=this.values,r=i.length;r===0&&(console.error("THREE.KeyframeTrack: Track is empty.",this),t=!1);let o=null;for(let a=0;a!==r;a++){let c=i[a];if(typeof c=="number"&&isNaN(c)){console.error("THREE.KeyframeTrack: Time is not a valid number.",this,a,c),t=!1;break}if(o!==null&&o>c){console.error("THREE.KeyframeTrack: Out of order keys.",this,a,c,o),t=!1;break}o=c}if(s!==void 0&&Cv(s))for(let a=0,c=s.length;a!==c;++a){let l=s[a];if(isNaN(l)){console.error("THREE.KeyframeTrack: Value is not a valid number.",this,a,l),t=!1;break}}return t}optimize(){let t=this.times.slice(),e=this.values.slice(),i=this.getValueSize(),s=this.getInterpolation()===Va,r=t.length-1,o=1;for(let a=1;a<r;++a){let c=!1,l=t[a],f=t[a+1];if(l!==f&&(a!==1||l!==t[0]))if(s)c=!0;else{let d=a*i,h=d-i,m=d+i;for(let x=0;x!==i;++x){let _=e[d+x];if(_!==e[h+x]||_!==e[m+x]){c=!0;break}}}if(c){if(a!==o){t[o]=t[a];let d=a*i,h=o*i;for(let m=0;m!==i;++m)e[h+m]=e[d+m]}++o}}if(r>0){t[o]=t[r];for(let a=r*i,c=o*i,l=0;l!==i;++l)e[c+l]=e[a+l];++o}return o!==t.length?(this.times=t.slice(0,o),this.values=e.slice(0,o*i)):(this.times=t,this.values=e),this}clone(){let t=this.times.slice(),e=this.values.slice(),i=this.constructor,s=new i(this.name,t,e);return s.createInterpolant=this.createInterpolant,s}};mn.prototype.TimeBufferType=Float32Array;mn.prototype.ValueBufferType=Float32Array;mn.prototype.DefaultInterpolation=nl;var Pi=class extends mn{constructor(t,e,i){super(t,e,i)}};Pi.prototype.ValueTypeName="bool";Pi.prototype.ValueBufferType=Array;Pi.prototype.DefaultInterpolation=po;Pi.prototype.InterpolantFactoryMethodLinear=void 0;Pi.prototype.InterpolantFactoryMethodSmooth=void 0;var wl=class extends mn{};wl.prototype.ValueTypeName="color";var Al=class extends mn{};Al.prototype.ValueTypeName="number";var El=class extends As{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,c=(i-e)/(s-e),l=t*a;for(let f=l+a;l!==f;l+=4)ke.slerpFlat(r,0,o,l-a,o,l,c);return r}},No=class extends mn{InterpolantFactoryMethodLinear(t){return new El(this.times,this.values,this.getValueSize(),t)}};No.prototype.ValueTypeName="quaternion";No.prototype.InterpolantFactoryMethodSmooth=void 0;var Ii=class extends mn{constructor(t,e,i){super(t,e,i)}};Ii.prototype.ValueTypeName="string";Ii.prototype.ValueBufferType=Array;Ii.prototype.DefaultInterpolation=po;Ii.prototype.InterpolantFactoryMethodLinear=void 0;Ii.prototype.InterpolantFactoryMethodSmooth=void 0;var Tl=class extends mn{};Tl.prototype.ValueTypeName="vector";var Cl=class{constructor(t,e,i){let s=this,r=!1,o=0,a=0,c,l=[];this.onStart=void 0,this.onLoad=t,this.onProgress=e,this.onError=i,this.itemStart=function(f){a++,r===!1&&s.onStart!==void 0&&s.onStart(f,o,a),r=!0},this.itemEnd=function(f){o++,s.onProgress!==void 0&&s.onProgress(f,o,a),o===a&&(r=!1,s.onLoad!==void 0&&s.onLoad())},this.itemError=function(f){s.onError!==void 0&&s.onError(f)},this.resolveURL=function(f){return c?c(f):f},this.setURLModifier=function(f){return c=f,this},this.addHandler=function(f,d){return l.push(f,d),this},this.removeHandler=function(f){let d=l.indexOf(f);return d!==-1&&l.splice(d,2),this},this.getHandler=function(f){for(let d=0,h=l.length;d<h;d+=2){let m=l[d],x=l[d+1];if(m.global&&(m.lastIndex=0),m.test(f))return x}return null}}},Rv=new Cl,Rl=class{constructor(t){this.manager=t!==void 0?t:Rv,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={}}load(){}loadAsync(t,e){let i=this;return new Promise(function(s,r){i.load(t,s,e,r)})}parse(){}setCrossOrigin(t){return this.crossOrigin=t,this}setWithCredentials(t){return this.withCredentials=t,this}setPath(t){return this.path=t,this}setResourcePath(t){return this.resourcePath=t,this}setRequestHeader(t){return this.requestHeader=t,this}};Rl.DEFAULT_MATERIAL_NAME="__DEFAULT";var Oo=class extends He{constructor(t,e=1){super(),this.isLight=!0,this.type="Light",this.color=new Ot(t),this.intensity=e}dispose(){}copy(t,e){return super.copy(t,e),this.color.copy(t.color),this.intensity=t.intensity,this}toJSON(t){let e=super.toJSON(t);return e.object.color=this.color.getHex(),e.object.intensity=this.intensity,this.groundColor!==void 0&&(e.object.groundColor=this.groundColor.getHex()),this.distance!==void 0&&(e.object.distance=this.distance),this.angle!==void 0&&(e.object.angle=this.angle),this.decay!==void 0&&(e.object.decay=this.decay),this.penumbra!==void 0&&(e.object.penumbra=this.penumbra),this.shadow!==void 0&&(e.object.shadow=this.shadow.toJSON()),this.target!==void 0&&(e.object.target=this.target.uuid),e}};var gc=new $t,Nu=new D,Ou=new D,Pl=class{constructor(t){this.camera=t,this.intensity=1,this.bias=0,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new yt(512,512),this.map=null,this.mapPass=null,this.matrix=new $t,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new ur,this._frameExtents=new yt(1,1),this._viewportCount=1,this._viewports=[new ce(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(t){let e=this.camera,i=this.matrix;Nu.setFromMatrixPosition(t.matrixWorld),e.position.copy(Nu),Ou.setFromMatrixPosition(t.target.matrixWorld),e.lookAt(Ou),e.updateMatrixWorld(),gc.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),this._frustum.setFromProjectionMatrix(gc),i.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),i.multiply(gc)}getViewport(t){return this._viewports[t]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(t){return this.camera=t.camera.clone(),this.intensity=t.intensity,this.bias=t.bias,this.radius=t.radius,this.mapSize.copy(t.mapSize),this}clone(){return new this.constructor().copy(this)}toJSON(){let t={};return this.intensity!==1&&(t.intensity=this.intensity),this.bias!==0&&(t.bias=this.bias),this.normalBias!==0&&(t.normalBias=this.normalBias),this.radius!==1&&(t.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(t.mapSize=this.mapSize.toArray()),t.camera=this.camera.toJSON(!1).object,delete t.camera.matrix,t}};var Il=class extends Pl{constructor(){super(new ws(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},fr=class extends Oo{constructor(t,e){super(t,e),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(He.DEFAULT_UP),this.updateMatrix(),this.target=new He,this.shadow=new Il}dispose(){this.shadow.dispose()}copy(t){return super.copy(t),this.target=t.target.clone(),this.shadow=t.shadow.clone(),this}},Fo=class extends Oo{constructor(t,e){super(t,e),this.isAmbientLight=!0,this.type="AmbientLight"}};var Bo=class{constructor(t=!0){this.autoStart=t,this.startTime=0,this.oldTime=0,this.elapsedTime=0,this.running=!1}start(){this.startTime=Fu(),this.oldTime=this.startTime,this.elapsedTime=0,this.running=!0}stop(){this.getElapsedTime(),this.running=!1,this.autoStart=!1}getElapsedTime(){return this.getDelta(),this.elapsedTime}getDelta(){let t=0;if(this.autoStart&&!this.running)return this.start(),0;if(this.running){let e=Fu();t=(e-this.oldTime)/1e3,this.oldTime=e,this.elapsedTime+=t}return t}};function Fu(){return performance.now()}var Xl="\\[\\]\\.:\\/",Pv=new RegExp("["+Xl+"]","g"),ql="[^"+Xl+"]",Iv="[^"+Xl.replace("\\.","")+"]",Lv=/((?:WC+[\/:])*)/.source.replace("WC",ql),Dv=/(WCOD+)?/.source.replace("WCOD",Iv),Uv=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",ql),Nv=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",ql),Ov=new RegExp("^"+Lv+Dv+Uv+Nv+"$"),Fv=["material","materials","bones","map"],Ll=class{constructor(t,e,i){let s=i||re.parseTrackName(e);this._targetGroup=t,this._bindings=t.subscribe_(e,s)}getValue(t,e){this.bind();let i=this._targetGroup.nCachedObjects_,s=this._bindings[i];s!==void 0&&s.getValue(t,e)}setValue(t,e){let i=this._bindings;for(let s=this._targetGroup.nCachedObjects_,r=i.length;s!==r;++s)i[s].setValue(t,e)}bind(){let t=this._bindings;for(let e=this._targetGroup.nCachedObjects_,i=t.length;e!==i;++e)t[e].bind()}unbind(){let t=this._bindings;for(let e=this._targetGroup.nCachedObjects_,i=t.length;e!==i;++e)t[e].unbind()}},re=class n{constructor(t,e,i){this.path=e,this.parsedPath=i||n.parseTrackName(e),this.node=n.findNode(t,this.parsedPath.nodeName),this.rootNode=t,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(t,e,i){return t&&t.isAnimationObjectGroup?new n.Composite(t,e,i):new n(t,e,i)}static sanitizeNodeName(t){return t.replace(/\s/g,"_").replace(Pv,"")}static parseTrackName(t){let e=Ov.exec(t);if(e===null)throw new Error("PropertyBinding: Cannot parse trackName: "+t);let i={nodeName:e[2],objectName:e[3],objectIndex:e[4],propertyName:e[5],propertyIndex:e[6]},s=i.nodeName&&i.nodeName.lastIndexOf(".");if(s!==void 0&&s!==-1){let r=i.nodeName.substring(s+1);Fv.indexOf(r)!==-1&&(i.nodeName=i.nodeName.substring(0,s),i.objectName=r)}if(i.propertyName===null||i.propertyName.length===0)throw new Error("PropertyBinding: can not parse propertyName from trackName: "+t);return i}static findNode(t,e){if(e===void 0||e===""||e==="."||e===-1||e===t.name||e===t.uuid)return t;if(t.skeleton){let i=t.skeleton.getBoneByName(e);if(i!==void 0)return i}if(t.children){let i=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===e||a.uuid===e)return a;let c=i(a.children);if(c)return c}return null},s=i(t.children);if(s)return s}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(t,e){t[e]=this.targetObject[this.propertyName]}_getValue_array(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)t[e++]=i[s]}_getValue_arrayElement(t,e){t[e]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(t,e){this.resolvedProperty.toArray(t,e)}_setValue_direct(t,e){this.targetObject[this.propertyName]=t[e]}_setValue_direct_setNeedsUpdate(t,e){this.targetObject[this.propertyName]=t[e],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(t,e){this.targetObject[this.propertyName]=t[e],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++]}_setValue_array_setNeedsUpdate(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(t,e){this.resolvedProperty[this.propertyIndex]=t[e]}_setValue_arrayElement_setNeedsUpdate(t,e){this.resolvedProperty[this.propertyIndex]=t[e],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(t,e){this.resolvedProperty[this.propertyIndex]=t[e],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(t,e){this.resolvedProperty.fromArray(t,e)}_setValue_fromArray_setNeedsUpdate(t,e){this.resolvedProperty.fromArray(t,e),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(t,e){this.resolvedProperty.fromArray(t,e),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(t,e){this.bind(),this.getValue(t,e)}_setValue_unbound(t,e){this.bind(),this.setValue(t,e)}bind(){let t=this.node,e=this.parsedPath,i=e.objectName,s=e.propertyName,r=e.propertyIndex;if(t||(t=n.findNode(this.rootNode,e.nodeName),this.node=t),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!t){console.warn("THREE.PropertyBinding: No target node found for track: "+this.path+".");return}if(i){let l=e.objectIndex;switch(i){case"materials":if(!t.material){console.error("THREE.PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!t.material.materials){console.error("THREE.PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}t=t.material.materials;break;case"bones":if(!t.skeleton){console.error("THREE.PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}t=t.skeleton.bones;for(let f=0;f<t.length;f++)if(t[f].name===l){l=f;break}break;case"map":if("map"in t){t=t.map;break}if(!t.material){console.error("THREE.PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!t.material.map){console.error("THREE.PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}t=t.material.map;break;default:if(t[i]===void 0){console.error("THREE.PropertyBinding: Can not bind to objectName of node undefined.",this);return}t=t[i]}if(l!==void 0){if(t[l]===void 0){console.error("THREE.PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,t);return}t=t[l]}}let o=t[s];if(o===void 0){let l=e.nodeName;console.error("THREE.PropertyBinding: Trying to update property for track: "+l+"."+s+" but it wasn't found.",t);return}let a=this.Versioning.None;this.targetObject=t,t.needsUpdate!==void 0?a=this.Versioning.NeedsUpdate:t.matrixWorldNeedsUpdate!==void 0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let c=this.BindingType.Direct;if(r!==void 0){if(s==="morphTargetInfluences"){if(!t.geometry){console.error("THREE.PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!t.geometry.morphAttributes){console.error("THREE.PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}t.morphTargetDictionary[r]!==void 0&&(r=t.morphTargetDictionary[r])}c=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(c=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(c=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=s;this.getValue=this.GetterByBindingType[c],this.setValue=this.SetterByBindingTypeAndVersioning[c][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};re.Composite=Ll;re.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};re.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};re.prototype.GetterByBindingType=[re.prototype._getValue_direct,re.prototype._getValue_array,re.prototype._getValue_arrayElement,re.prototype._getValue_toArray];re.prototype.SetterByBindingTypeAndVersioning=[[re.prototype._setValue_direct,re.prototype._setValue_direct_setNeedsUpdate,re.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[re.prototype._setValue_array,re.prototype._setValue_array_setNeedsUpdate,re.prototype._setValue_array_setMatrixWorldNeedsUpdate],[re.prototype._setValue_arrayElement,re.prototype._setValue_arrayElement_setNeedsUpdate,re.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[re.prototype._setValue_fromArray,re.prototype._setValue_fromArray_setNeedsUpdate,re.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var ay=new Float32Array(1);var Bu=new $t,zo=class{constructor(t,e,i=0,s=1/0){this.ray=new bo(t,e),this.near=i,this.far=s,this.camera=null,this.layers=new lr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(t,e){this.ray.set(t,e)}setFromCamera(t,e){e.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(t.x,t.y,.5).unproject(e).sub(this.ray.origin).normalize(),this.camera=e):e.isOrthographicCamera?(this.ray.origin.set(t.x,t.y,(e.near+e.far)/(e.near-e.far)).unproject(e),this.ray.direction.set(0,0,-1).transformDirection(e.matrixWorld),this.camera=e):console.error("THREE.Raycaster: Unsupported camera type: "+e.type)}setFromXRController(t){return Bu.identity().extractRotation(t.matrixWorld),this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(Bu),this}intersectObject(t,e=!0,i=[]){return Dl(t,this,i,e),i.sort(zu),i}intersectObjects(t,e=!0,i=[]){for(let s=0,r=t.length;s<r;s++)Dl(t[s],this,i,e);return i.sort(zu),i}};function zu(n,t){return n.distance-t.distance}function Dl(n,t,e,i){let s=!0;if(n.layers.test(t.layers)&&n.raycast(t,e)===!1&&(s=!1),s===!0&&i===!0){let r=n.children;for(let o=0,a=r.length;o<a;o++)Dl(r[o],t,e,!0)}}var ko=class extends Bn{constructor(t,e=null){super(),this.object=t,this.domElement=e,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(){}disconnect(){}dispose(){}update(){}};typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"169"}}));typeof window<"u"&&(window.__THREE__?console.warn("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="169");var Yl={type:"change"},Kl={type:"start"},Jl={type:"end"},cd=1e-6,Yt={NONE:-1,ROTATE:0,ZOOM:1,PAN:2,TOUCH_ROTATE:3,TOUCH_ZOOM_PAN:4},Wo=new yt,ri=new yt,zv=new D,Xo=new D,Zl=new D,Cs=new ke,ld=new D,qo=new D,jl=new D,Yo=new D,Zo=class extends ko{constructor(t,e=null){super(t,e),this.enabled=!0,this.screen={left:0,top:0,width:0,height:0},this.rotateSpeed=1,this.zoomSpeed=1.2,this.panSpeed=.3,this.noRotate=!1,this.noZoom=!1,this.noPan=!1,this.staticMoving=!1,this.dynamicDampingFactor=.2,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.keys=["KeyA","KeyS","KeyD"],this.mouseButtons={LEFT:Li.ROTATE,MIDDLE:Li.DOLLY,RIGHT:Li.PAN},this.state=Yt.NONE,this.keyState=Yt.NONE,this.target=new D,this._lastPosition=new D,this._lastZoom=1,this._touchZoomDistanceStart=0,this._touchZoomDistanceEnd=0,this._lastAngle=0,this._eye=new D,this._movePrev=new yt,this._moveCurr=new yt,this._lastAxis=new D,this._zoomStart=new yt,this._zoomEnd=new yt,this._panStart=new yt,this._panEnd=new yt,this._pointers=[],this._pointerPositions={},this._onPointerMove=Hv.bind(this),this._onPointerDown=kv.bind(this),this._onPointerUp=Vv.bind(this),this._onPointerCancel=Gv.bind(this),this._onContextMenu=Kv.bind(this),this._onMouseWheel=jv.bind(this),this._onKeyDown=Xv.bind(this),this._onKeyUp=Wv.bind(this),this._onTouchStart=Jv.bind(this),this._onTouchMove=Qv.bind(this),this._onTouchEnd=$v.bind(this),this._onMouseDown=qv.bind(this),this._onMouseMove=Yv.bind(this),this._onMouseUp=Zv.bind(this),this._target0=this.target.clone(),this._position0=this.object.position.clone(),this._up0=this.object.up.clone(),this._zoom0=this.object.zoom,e!==null&&(this.connect(),this.handleResize()),this.update()}connect(){window.addEventListener("keydown",this._onKeyDown),window.addEventListener("keyup",this._onKeyUp),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerCancel),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.style.touchAction="none"}disconnect(){window.removeEventListener("keydown",this._onKeyDown),window.removeEventListener("keyup",this._onKeyUp),this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.removeEventListener("pointermove",this._onPointerMove),this.domElement.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerCancel),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.domElement.style.touchAction="auto"}dispose(){this.disconnect()}handleResize(){let t=this.domElement.getBoundingClientRect(),e=this.domElement.ownerDocument.documentElement;this.screen.left=t.left+window.pageXOffset-e.clientLeft,this.screen.top=t.top+window.pageYOffset-e.clientTop,this.screen.width=t.width,this.screen.height=t.height}update(){this._eye.subVectors(this.object.position,this.target),this.noRotate||this._rotateCamera(),this.noZoom||this._zoomCamera(),this.noPan||this._panCamera(),this.object.position.addVectors(this.target,this._eye),this.object.isPerspectiveCamera?(this._checkDistances(),this.object.lookAt(this.target),this._lastPosition.distanceToSquared(this.object.position)>cd&&(this.dispatchEvent(Yl),this._lastPosition.copy(this.object.position))):this.object.isOrthographicCamera?(this.object.lookAt(this.target),(this._lastPosition.distanceToSquared(this.object.position)>cd||this._lastZoom!==this.object.zoom)&&(this.dispatchEvent(Yl),this._lastPosition.copy(this.object.position),this._lastZoom=this.object.zoom)):console.warn("THREE.TrackballControls: Unsupported camera type.")}reset(){this.state=Yt.NONE,this.keyState=Yt.NONE,this.target.copy(this._target0),this.object.position.copy(this._position0),this.object.up.copy(this._up0),this.object.zoom=this._zoom0,this.object.updateProjectionMatrix(),this._eye.subVectors(this.object.position,this.target),this.object.lookAt(this.target),this.dispatchEvent(Yl),this._lastPosition.copy(this.object.position),this._lastZoom=this.object.zoom}_panCamera(){if(ri.copy(this._panEnd).sub(this._panStart),ri.lengthSq()){if(this.object.isOrthographicCamera){let t=(this.object.right-this.object.left)/this.object.zoom/this.domElement.clientWidth,e=(this.object.top-this.object.bottom)/this.object.zoom/this.domElement.clientWidth;ri.x*=t,ri.y*=e}ri.multiplyScalar(this._eye.length()*this.panSpeed),Xo.copy(this._eye).cross(this.object.up).setLength(ri.x),Xo.add(zv.copy(this.object.up).setLength(ri.y)),this.object.position.add(Xo),this.target.add(Xo),this.staticMoving?this._panStart.copy(this._panEnd):this._panStart.add(ri.subVectors(this._panEnd,this._panStart).multiplyScalar(this.dynamicDampingFactor))}}_rotateCamera(){Yo.set(this._moveCurr.x-this._movePrev.x,this._moveCurr.y-this._movePrev.y,0);let t=Yo.length();t?(this._eye.copy(this.object.position).sub(this.target),ld.copy(this._eye).normalize(),qo.copy(this.object.up).normalize(),jl.crossVectors(qo,ld).normalize(),qo.setLength(this._moveCurr.y-this._movePrev.y),jl.setLength(this._moveCurr.x-this._movePrev.x),Yo.copy(qo.add(jl)),Zl.crossVectors(Yo,this._eye).normalize(),t*=this.rotateSpeed,Cs.setFromAxisAngle(Zl,t),this._eye.applyQuaternion(Cs),this.object.up.applyQuaternion(Cs),this._lastAxis.copy(Zl),this._lastAngle=t):!this.staticMoving&&this._lastAngle&&(this._lastAngle*=Math.sqrt(1-this.dynamicDampingFactor),this._eye.copy(this.object.position).sub(this.target),Cs.setFromAxisAngle(this._lastAxis,this._lastAngle),this._eye.applyQuaternion(Cs),this.object.up.applyQuaternion(Cs)),this._movePrev.copy(this._moveCurr)}_zoomCamera(){let t;this.state===Yt.TOUCH_ZOOM_PAN?(t=this._touchZoomDistanceStart/this._touchZoomDistanceEnd,this._touchZoomDistanceStart=this._touchZoomDistanceEnd,this.object.isPerspectiveCamera?this._eye.multiplyScalar(t):this.object.isOrthographicCamera?(this.object.zoom=Gl.clamp(this.object.zoom/t,this.minZoom,this.maxZoom),this._lastZoom!==this.object.zoom&&this.object.updateProjectionMatrix()):console.warn("THREE.TrackballControls: Unsupported camera type")):(t=1+(this._zoomEnd.y-this._zoomStart.y)*this.zoomSpeed,t!==1&&t>0&&(this.object.isPerspectiveCamera?this._eye.multiplyScalar(t):this.object.isOrthographicCamera?(this.object.zoom=Gl.clamp(this.object.zoom/t,this.minZoom,this.maxZoom),this._lastZoom!==this.object.zoom&&this.object.updateProjectionMatrix()):console.warn("THREE.TrackballControls: Unsupported camera type")),this.staticMoving?this._zoomStart.copy(this._zoomEnd):this._zoomStart.y+=(this._zoomEnd.y-this._zoomStart.y)*this.dynamicDampingFactor)}_getMouseOnScreen(t,e){return Wo.set((t-this.screen.left)/this.screen.width,(e-this.screen.top)/this.screen.height),Wo}_getMouseOnCircle(t,e){return Wo.set((t-this.screen.width*.5-this.screen.left)/(this.screen.width*.5),(this.screen.height+2*(this.screen.top-e))/this.screen.width),Wo}_addPointer(t){this._pointers.push(t)}_removePointer(t){delete this._pointerPositions[t.pointerId];for(let e=0;e<this._pointers.length;e++)if(this._pointers[e].pointerId==t.pointerId){this._pointers.splice(e,1);return}}_trackPointer(t){let e=this._pointerPositions[t.pointerId];e===void 0&&(e=new yt,this._pointerPositions[t.pointerId]=e),e.set(t.pageX,t.pageY)}_getSecondPointerPosition(t){let e=t.pointerId===this._pointers[0].pointerId?this._pointers[1]:this._pointers[0];return this._pointerPositions[e.pointerId]}_checkDistances(){(!this.noZoom||!this.noPan)&&(this._eye.lengthSq()>this.maxDistance*this.maxDistance&&(this.object.position.addVectors(this.target,this._eye.setLength(this.maxDistance)),this._zoomStart.copy(this._zoomEnd)),this._eye.lengthSq()<this.minDistance*this.minDistance&&(this.object.position.addVectors(this.target,this._eye.setLength(this.minDistance)),this._zoomStart.copy(this._zoomEnd)))}};function kv(n){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(n.pointerId),this.domElement.addEventListener("pointermove",this._onPointerMove),this.domElement.addEventListener("pointerup",this._onPointerUp)),this._addPointer(n),n.pointerType==="touch"?this._onTouchStart(n):this._onMouseDown(n))}function Hv(n){this.enabled!==!1&&(n.pointerType==="touch"?this._onTouchMove(n):this._onMouseMove(n))}function Vv(n){this.enabled!==!1&&(n.pointerType==="touch"?this._onTouchEnd(n):this._onMouseUp(),this._removePointer(n),this._pointers.length===0&&(this.domElement.releasePointerCapture(n.pointerId),this.domElement.removeEventListener("pointermove",this._onPointerMove),this.domElement.removeEventListener("pointerup",this._onPointerUp)))}function Gv(n){this._removePointer(n)}function Wv(){this.enabled!==!1&&(this.keyState=Yt.NONE,window.addEventListener("keydown",this._onKeyDown))}function Xv(n){this.enabled!==!1&&(window.removeEventListener("keydown",this._onKeyDown),this.keyState===Yt.NONE&&(n.code===this.keys[Yt.ROTATE]&&!this.noRotate?this.keyState=Yt.ROTATE:n.code===this.keys[Yt.ZOOM]&&!this.noZoom?this.keyState=Yt.ZOOM:n.code===this.keys[Yt.PAN]&&!this.noPan&&(this.keyState=Yt.PAN)))}function qv(n){let t;switch(n.button){case 0:t=this.mouseButtons.LEFT;break;case 1:t=this.mouseButtons.MIDDLE;break;case 2:t=this.mouseButtons.RIGHT;break;default:t=-1}switch(t){case Li.DOLLY:this.state=Yt.ZOOM;break;case Li.ROTATE:this.state=Yt.ROTATE;break;case Li.PAN:this.state=Yt.PAN;break;default:this.state=Yt.NONE}let e=this.keyState!==Yt.NONE?this.keyState:this.state;e===Yt.ROTATE&&!this.noRotate?(this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY)),this._movePrev.copy(this._moveCurr)):e===Yt.ZOOM&&!this.noZoom?(this._zoomStart.copy(this._getMouseOnScreen(n.pageX,n.pageY)),this._zoomEnd.copy(this._zoomStart)):e===Yt.PAN&&!this.noPan&&(this._panStart.copy(this._getMouseOnScreen(n.pageX,n.pageY)),this._panEnd.copy(this._panStart)),this.dispatchEvent(Kl)}function Yv(n){let t=this.keyState!==Yt.NONE?this.keyState:this.state;t===Yt.ROTATE&&!this.noRotate?(this._movePrev.copy(this._moveCurr),this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY))):t===Yt.ZOOM&&!this.noZoom?this._zoomEnd.copy(this._getMouseOnScreen(n.pageX,n.pageY)):t===Yt.PAN&&!this.noPan&&this._panEnd.copy(this._getMouseOnScreen(n.pageX,n.pageY))}function Zv(){this.state=Yt.NONE,this.dispatchEvent(Jl)}function jv(n){if(this.enabled!==!1&&this.noZoom!==!0){switch(n.preventDefault(),n.deltaMode){case 2:this._zoomStart.y-=n.deltaY*.025;break;case 1:this._zoomStart.y-=n.deltaY*.01;break;default:this._zoomStart.y-=n.deltaY*25e-5;break}this.dispatchEvent(Kl),this.dispatchEvent(Jl)}}function Kv(n){this.enabled!==!1&&n.preventDefault()}function Jv(n){if(this._trackPointer(n),this._pointers.length===1)this.state=Yt.TOUCH_ROTATE,this._moveCurr.copy(this._getMouseOnCircle(this._pointers[0].pageX,this._pointers[0].pageY)),this._movePrev.copy(this._moveCurr);else{this.state=Yt.TOUCH_ZOOM_PAN;let t=this._pointers[0].pageX-this._pointers[1].pageX,e=this._pointers[0].pageY-this._pointers[1].pageY;this._touchZoomDistanceEnd=this._touchZoomDistanceStart=Math.sqrt(t*t+e*e);let i=(this._pointers[0].pageX+this._pointers[1].pageX)/2,s=(this._pointers[0].pageY+this._pointers[1].pageY)/2;this._panStart.copy(this._getMouseOnScreen(i,s)),this._panEnd.copy(this._panStart)}this.dispatchEvent(Kl)}function Qv(n){if(this._trackPointer(n),this._pointers.length===1)this._movePrev.copy(this._moveCurr),this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY));else{let t=this._getSecondPointerPosition(n),e=n.pageX-t.x,i=n.pageY-t.y;this._touchZoomDistanceEnd=Math.sqrt(e*e+i*i);let s=(n.pageX+t.x)/2,r=(n.pageY+t.y)/2;this._panEnd.copy(this._getMouseOnScreen(s,r))}}function $v(n){switch(this._pointers.length){case 0:this.state=Yt.NONE;break;case 1:this.state=Yt.TOUCH_ROTATE,this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY)),this._movePrev.copy(this._moveCurr);break;case 2:this.state=Yt.TOUCH_ZOOM_PAN;for(let t=0;t<this._pointers.length;t++)if(this._pointers[t].pointerId!==n.pointerId){let e=this._pointerPositions[this._pointers[t].pointerId];this._moveCurr.copy(this._getMouseOnCircle(e.x,e.y)),this._movePrev.copy(this._moveCurr);break}break}this.dispatchEvent(Jl)}var jo={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform float opacity;

		uniform sampler2D tDiffuse;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );
			gl_FragColor = opacity * texel;


		}`};var Je=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},t_=new ws(-1,1,1,-1,0,1),Ql=class extends pn{constructor(){super(),this.setAttribute("position",new be([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new be([0,2,0,0,2,0],2))}},e_=new Ql,oi=class{constructor(t){this._mesh=new Oe(e_,t)}dispose(){this._mesh.geometry.dispose()}render(t){t.render(this._mesh,t_)}get material(){return this._mesh.material}set material(t){this._mesh.material=t}};var Ko=class extends Je{constructor(t,e){super(),this.textureID=e!==void 0?e:"tDiffuse",t instanceof te?(this.uniforms=t.uniforms,this.material=t):t&&(this.uniforms=wn.clone(t.uniforms),this.material=new te({name:t.name!==void 0?t.name:"unspecified",defines:Object.assign({},t.defines),uniforms:this.uniforms,vertexShader:t.vertexShader,fragmentShader:t.fragmentShader})),this.fsQuad=new oi(this.material)}render(t,e,i){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=i.texture),this.fsQuad.material=this.material,this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(e),this.clear&&t.clear(t.autoClearColor,t.autoClearDepth,t.autoClearStencil),this.fsQuad.render(t))}dispose(){this.material.dispose(),this.fsQuad.dispose()}};var pr=class extends Je{constructor(t,e){super(),this.scene=t,this.camera=e,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(t,e,i){let s=t.getContext(),r=t.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(s.REPLACE,s.REPLACE,s.REPLACE),r.buffers.stencil.setFunc(s.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),t.setRenderTarget(i),this.clear&&t.clear(),t.render(this.scene,this.camera),t.setRenderTarget(e),this.clear&&t.clear(),t.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(s.EQUAL,1,4294967295),r.buffers.stencil.setOp(s.KEEP,s.KEEP,s.KEEP),r.buffers.stencil.setLocked(!0)}},Jo=class extends Je{constructor(){super(),this.needsSwap=!1}render(t){t.state.buffers.stencil.setLocked(!1),t.state.buffers.stencil.setTest(!1)}};var Qo=class{constructor(t,e){if(this.renderer=t,this._pixelRatio=t.getPixelRatio(),e===void 0){let i=t.getSize(new yt);this._width=i.width,this._height=i.height,e=new fe(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Ve}),e.texture.name="EffectComposer.rt1"}else this._width=e.width,this._height=e.height;this.renderTarget1=e,this.renderTarget2=e.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Ko(jo),this.copyPass.material.blending=Sn,this.clock=new Bo}swapBuffers(){let t=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=t}addPass(t){this.passes.push(t),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(t,e){this.passes.splice(e,0,t),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(t){let e=this.passes.indexOf(t);e!==-1&&this.passes.splice(e,1)}isLastEnabledPass(t){for(let e=t+1;e<this.passes.length;e++)if(this.passes[e].enabled)return!1;return!0}render(t){t===void 0&&(t=this.clock.getDelta());let e=this.renderer.getRenderTarget(),i=!1;for(let s=0,r=this.passes.length;s<r;s++){let o=this.passes[s];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(s),o.render(this.renderer,this.writeBuffer,this.readBuffer,t,i),o.needsSwap){if(i){let a=this.renderer.getContext(),c=this.renderer.state.buffers.stencil;c.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,t),c.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}pr!==void 0&&(o instanceof pr?i=!0:o instanceof Jo&&(i=!1))}}this.renderer.setRenderTarget(e)}reset(t){if(t===void 0){let e=this.renderer.getSize(new yt);this._pixelRatio=this.renderer.getPixelRatio(),this._width=e.width,this._height=e.height,t=this.renderTarget1.clone(),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=t,this.renderTarget2=t.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(t,e){this._width=t,this._height=e;let i=this._width*this._pixelRatio,s=this._height*this._pixelRatio;this.renderTarget1.setSize(i,s),this.renderTarget2.setSize(i,s);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(i,s)}setPixelRatio(t){this._pixelRatio=t,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var $o=class extends Je{constructor(t,e,i=null,s=null,r=null){super(),this.scene=t,this.camera=e,this.overrideMaterial=i,this.clearColor=s,this.clearAlpha=r,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this._oldClearColor=new Ot}render(t,e,i){let s=t.autoClear;t.autoClear=!1;let r,o;this.overrideMaterial!==null&&(o=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(t.getClearColor(this._oldClearColor),t.setClearColor(this.clearColor,t.getClearAlpha())),this.clearAlpha!==null&&(r=t.getClearAlpha(),t.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&t.clearDepth(),t.setRenderTarget(this.renderToScreen?null:i),this.clear===!0&&t.clear(t.autoClearColor,t.autoClearDepth,t.autoClearStencil),t.render(this.scene,this.camera),this.clearColor!==null&&t.setClearColor(this._oldClearColor),this.clearAlpha!==null&&t.setClearAlpha(r),this.overrideMaterial!==null&&(this.scene.overrideMaterial=o),t.autoClear=s}};var hd={name:"LuminosityHighPassShader",shaderID:"luminosityHighPass",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Ot(0)},defaultOpacity:{value:0}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;
		uniform vec3 defaultColor;
		uniform float defaultOpacity;
		uniform float luminosityThreshold;
		uniform float smoothWidth;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );

			float v = luminance( texel.xyz );

			vec4 outputColor = vec4( defaultColor.rgb, defaultOpacity );

			float alpha = smoothstep( luminosityThreshold, luminosityThreshold + smoothWidth, v );

			gl_FragColor = mix( outputColor, texel, alpha );

		}`};var Rs=class n extends Je{constructor(t,e,i,s){super(),this.strength=e!==void 0?e:1,this.radius=i,this.threshold=s,this.resolution=t!==void 0?new yt(t.x,t.y):new yt(256,256),this.clearColor=new Ot(0,0,0),this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new fe(r,o,{type:Ve}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let d=0;d<this.nMips;d++){let h=new fe(r,o,{type:Ve});h.texture.name="UnrealBloomPass.h"+d,h.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(h);let m=new fe(r,o,{type:Ve});m.texture.name="UnrealBloomPass.v"+d,m.texture.generateMipmaps=!1,this.renderTargetsVertical.push(m),r=Math.round(r/2),o=Math.round(o/2)}let a=hd;this.highPassUniforms=wn.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=s,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new te({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let c=[3,5,7,9,11];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let d=0;d<this.nMips;d++)this.separableBlurMaterials.push(this.getSeperableBlurMaterial(c[d])),this.separableBlurMaterials[d].uniforms.invSize.value=new yt(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this.getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=e,this.compositeMaterial.uniforms.bloomRadius.value=.1;let l=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=l,this.bloomTintColors=[new D(1,1,1),new D(1,1,1),new D(1,1,1),new D(1,1,1),new D(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors;let f=jo;this.copyUniforms=wn.clone(f.uniforms),this.blendMaterial=new te({uniforms:this.copyUniforms,vertexShader:f.vertexShader,fragmentShader:f.fragmentShader,blending:ni,depthTest:!1,depthWrite:!1,transparent:!0}),this.enabled=!0,this.needsSwap=!1,this._oldClearColor=new Ot,this.oldClearAlpha=1,this.basic=new bs,this.fsQuad=new oi(null)}dispose(){for(let t=0;t<this.renderTargetsHorizontal.length;t++)this.renderTargetsHorizontal[t].dispose();for(let t=0;t<this.renderTargetsVertical.length;t++)this.renderTargetsVertical[t].dispose();this.renderTargetBright.dispose();for(let t=0;t<this.separableBlurMaterials.length;t++)this.separableBlurMaterials[t].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this.basic.dispose(),this.fsQuad.dispose()}setSize(t,e){let i=Math.round(t/2),s=Math.round(e/2);this.renderTargetBright.setSize(i,s);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(i,s),this.renderTargetsVertical[r].setSize(i,s),this.separableBlurMaterials[r].uniforms.invSize.value=new yt(1/i,1/s),i=Math.round(i/2),s=Math.round(s/2)}render(t,e,i,s,r){t.getClearColor(this._oldClearColor),this.oldClearAlpha=t.getClearAlpha();let o=t.autoClear;t.autoClear=!1,t.setClearColor(this.clearColor,0),r&&t.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this.fsQuad.material=this.basic,this.basic.map=i.texture,t.setRenderTarget(null),t.clear(),this.fsQuad.render(t)),this.highPassUniforms.tDiffuse.value=i.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this.fsQuad.material=this.materialHighPassFilter,t.setRenderTarget(this.renderTargetBright),t.clear(),this.fsQuad.render(t);let a=this.renderTargetBright;for(let c=0;c<this.nMips;c++)this.fsQuad.material=this.separableBlurMaterials[c],this.separableBlurMaterials[c].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[c].uniforms.direction.value=n.BlurDirectionX,t.setRenderTarget(this.renderTargetsHorizontal[c]),t.clear(),this.fsQuad.render(t),this.separableBlurMaterials[c].uniforms.colorTexture.value=this.renderTargetsHorizontal[c].texture,this.separableBlurMaterials[c].uniforms.direction.value=n.BlurDirectionY,t.setRenderTarget(this.renderTargetsVertical[c]),t.clear(),this.fsQuad.render(t),a=this.renderTargetsVertical[c];this.fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,t.setRenderTarget(this.renderTargetsHorizontal[0]),t.clear(),this.fsQuad.render(t),this.fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&t.state.buffers.stencil.setTest(!0),this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(i),this.fsQuad.render(t)),t.setClearColor(this._oldClearColor,this.oldClearAlpha),t.autoClear=o}getSeperableBlurMaterial(t){let e=[];for(let i=0;i<t;i++)e.push(.39894*Math.exp(-.5*i*i/(t*t))/t);return new te({defines:{KERNEL_RADIUS:t},uniforms:{colorTexture:{value:null},invSize:{value:new yt(.5,.5)},direction:{value:new yt(.5,.5)},gaussianCoefficients:{value:e}},vertexShader:`varying vec2 vUv;
				void main() {
					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
				}`,fragmentShader:`#include <common>
				varying vec2 vUv;
				uniform sampler2D colorTexture;
				uniform vec2 invSize;
				uniform vec2 direction;
				uniform float gaussianCoefficients[KERNEL_RADIUS];

				void main() {
					float weightSum = gaussianCoefficients[0];
					vec3 diffuseSum = texture2D( colorTexture, vUv ).rgb * weightSum;
					for( int i = 1; i < KERNEL_RADIUS; i ++ ) {
						float x = float(i);
						float w = gaussianCoefficients[i];
						vec2 uvOffset = direction * invSize * x;
						vec3 sample1 = texture2D( colorTexture, vUv + uvOffset ).rgb;
						vec3 sample2 = texture2D( colorTexture, vUv - uvOffset ).rgb;
						diffuseSum += (sample1 + sample2) * w;
						weightSum += 2.0 * w;
					}
					gl_FragColor = vec4(diffuseSum/weightSum, 1.0);
				}`})}getCompositeMaterial(t){return new te({defines:{NUM_MIPS:t},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`varying vec2 vUv;
				void main() {
					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
				}`,fragmentShader:`varying vec2 vUv;
				uniform sampler2D blurTexture1;
				uniform sampler2D blurTexture2;
				uniform sampler2D blurTexture3;
				uniform sampler2D blurTexture4;
				uniform sampler2D blurTexture5;
				uniform float bloomStrength;
				uniform float bloomRadius;
				uniform float bloomFactors[NUM_MIPS];
				uniform vec3 bloomTintColors[NUM_MIPS];

				float lerpBloomFactor(const in float factor) {
					float mirrorFactor = 1.2 - factor;
					return mix(factor, mirrorFactor, bloomRadius);
				}

				void main() {
					gl_FragColor = bloomStrength * ( lerpBloomFactor(bloomFactors[0]) * vec4(bloomTintColors[0], 1.0) * texture2D(blurTexture1, vUv) +
						lerpBloomFactor(bloomFactors[1]) * vec4(bloomTintColors[1], 1.0) * texture2D(blurTexture2, vUv) +
						lerpBloomFactor(bloomFactors[2]) * vec4(bloomTintColors[2], 1.0) * texture2D(blurTexture3, vUv) +
						lerpBloomFactor(bloomFactors[3]) * vec4(bloomTintColors[3], 1.0) * texture2D(blurTexture4, vUv) +
						lerpBloomFactor(bloomFactors[4]) * vec4(bloomTintColors[4], 1.0) * texture2D(blurTexture5, vUv) );
				}`})}};Rs.BlurDirectionX=new yt(1,0);Rs.BlurDirectionY=new yt(0,1);var mr={name:"SMAAEdgesShader",defines:{SMAA_THRESHOLD:"0.1"},uniforms:{tDiffuse:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];

		void SMAAEdgeDetectionVS( vec2 texcoord ) {
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -1.0, 0.0, 0.0,  1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4(  1.0, 0.0, 0.0, -1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 2 ] = texcoord.xyxy + resolution.xyxy * vec4( -2.0, 0.0, 0.0,  2.0 ); // WebGL port note: Changed sign in W component
		}

		void main() {

			vUv = uv;

			SMAAEdgeDetectionVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];

		vec4 SMAAColorEdgeDetectionPS( vec2 texcoord, vec4 offset[3], sampler2D colorTex ) {
			vec2 threshold = vec2( SMAA_THRESHOLD, SMAA_THRESHOLD );

			// Calculate color deltas:
			vec4 delta;
			vec3 C = texture2D( colorTex, texcoord ).rgb;

			vec3 Cleft = texture2D( colorTex, offset[0].xy ).rgb;
			vec3 t = abs( C - Cleft );
			delta.x = max( max( t.r, t.g ), t.b );

			vec3 Ctop = texture2D( colorTex, offset[0].zw ).rgb;
			t = abs( C - Ctop );
			delta.y = max( max( t.r, t.g ), t.b );

			// We do the usual threshold:
			vec2 edges = step( threshold, delta.xy );

			// Then discard if there is no edge:
			if ( dot( edges, vec2( 1.0, 1.0 ) ) == 0.0 )
				discard;

			// Calculate right and bottom deltas:
			vec3 Cright = texture2D( colorTex, offset[1].xy ).rgb;
			t = abs( C - Cright );
			delta.z = max( max( t.r, t.g ), t.b );

			vec3 Cbottom  = texture2D( colorTex, offset[1].zw ).rgb;
			t = abs( C - Cbottom );
			delta.w = max( max( t.r, t.g ), t.b );

			// Calculate the maximum delta in the direct neighborhood:
			float maxDelta = max( max( max( delta.x, delta.y ), delta.z ), delta.w );

			// Calculate left-left and top-top deltas:
			vec3 Cleftleft  = texture2D( colorTex, offset[2].xy ).rgb;
			t = abs( C - Cleftleft );
			delta.z = max( max( t.r, t.g ), t.b );

			vec3 Ctoptop = texture2D( colorTex, offset[2].zw ).rgb;
			t = abs( C - Ctoptop );
			delta.w = max( max( t.r, t.g ), t.b );

			// Calculate the final maximum delta:
			maxDelta = max( max( maxDelta, delta.z ), delta.w );

			// Local contrast adaptation in action:
			edges.xy *= step( 0.5 * maxDelta, delta.xy );

			return vec4( edges, 0.0, 0.0 );
		}

		void main() {

			gl_FragColor = SMAAColorEdgeDetectionPS( vUv, vOffset, tDiffuse );

		}`},gr={name:"SMAAWeightsShader",defines:{SMAA_MAX_SEARCH_STEPS:"8",SMAA_AREATEX_MAX_DISTANCE:"16",SMAA_AREATEX_PIXEL_SIZE:"( 1.0 / vec2( 160.0, 560.0 ) )",SMAA_AREATEX_SUBTEX_SIZE:"( 1.0 / 7.0 )"},uniforms:{tDiffuse:{value:null},tArea:{value:null},tSearch:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];
		varying vec2 vPixcoord;

		void SMAABlendingWeightCalculationVS( vec2 texcoord ) {
			vPixcoord = texcoord / resolution;

			// We will use these offsets for the searches later on (see @PSEUDO_GATHER4):
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -0.25, 0.125, 1.25, 0.125 ); // WebGL port note: Changed sign in Y and W components
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4( -0.125, 0.25, -0.125, -1.25 ); // WebGL port note: Changed sign in Y and W components

			// And these for the searches, they indicate the ends of the loops:
			vOffset[ 2 ] = vec4( vOffset[ 0 ].xz, vOffset[ 1 ].yw ) + vec4( -2.0, 2.0, -2.0, 2.0 ) * resolution.xxyy * float( SMAA_MAX_SEARCH_STEPS );

		}

		void main() {

			vUv = uv;

			SMAABlendingWeightCalculationVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		#define SMAASampleLevelZeroOffset( tex, coord, offset ) texture2D( tex, coord + float( offset ) * resolution, 0.0 )

		uniform sampler2D tDiffuse;
		uniform sampler2D tArea;
		uniform sampler2D tSearch;
		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[3];
		varying vec2 vPixcoord;

		#if __VERSION__ == 100
		vec2 round( vec2 x ) {
			return sign( x ) * floor( abs( x ) + 0.5 );
		}
		#endif

		float SMAASearchLength( sampler2D searchTex, vec2 e, float bias, float scale ) {
			// Not required if searchTex accesses are set to point:
			// float2 SEARCH_TEX_PIXEL_SIZE = 1.0 / float2(66.0, 33.0);
			// e = float2(bias, 0.0) + 0.5 * SEARCH_TEX_PIXEL_SIZE +
			//     e * float2(scale, 1.0) * float2(64.0, 32.0) * SEARCH_TEX_PIXEL_SIZE;
			e.r = bias + e.r * scale;
			return 255.0 * texture2D( searchTex, e, 0.0 ).r;
		}

		float SMAASearchXLeft( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			/**
				* @PSEUDO_GATHER4
				* This texcoord has been offset by (-0.25, -0.125) in the vertex shader to
				* sample between edge, thus fetching four edges in a row.
				* Sampling with different offsets in each direction allows to disambiguate
				* which edges are active from the four fetched ones.
				*/
			vec2 e = vec2( 0.0, 1.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord -= vec2( 2.0, 0.0 ) * resolution;
				if ( ! ( texcoord.x > end && e.g > 0.8281 && e.r == 0.0 ) ) break;
			}

			// We correct the previous (-0.25, -0.125) offset we applied:
			texcoord.x += 0.25 * resolution.x;

			// The searches are bias by 1, so adjust the coords accordingly:
			texcoord.x += resolution.x;

			// Disambiguate the length added by the last step:
			texcoord.x += 2.0 * resolution.x; // Undo last step
			texcoord.x -= resolution.x * SMAASearchLength(searchTex, e, 0.0, 0.5);

			return texcoord.x;
		}

		float SMAASearchXRight( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 0.0, 1.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord += vec2( 2.0, 0.0 ) * resolution;
				if ( ! ( texcoord.x < end && e.g > 0.8281 && e.r == 0.0 ) ) break;
			}

			texcoord.x -= 0.25 * resolution.x;
			texcoord.x -= resolution.x;
			texcoord.x -= 2.0 * resolution.x;
			texcoord.x += resolution.x * SMAASearchLength( searchTex, e, 0.5, 0.5 );

			return texcoord.x;
		}

		float SMAASearchYUp( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 1.0, 0.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord += vec2( 0.0, 2.0 ) * resolution; // WebGL port note: Changed sign
				if ( ! ( texcoord.y > end && e.r > 0.8281 && e.g == 0.0 ) ) break;
			}

			texcoord.y -= 0.25 * resolution.y; // WebGL port note: Changed sign
			texcoord.y -= resolution.y; // WebGL port note: Changed sign
			texcoord.y -= 2.0 * resolution.y; // WebGL port note: Changed sign
			texcoord.y += resolution.y * SMAASearchLength( searchTex, e.gr, 0.0, 0.5 ); // WebGL port note: Changed sign

			return texcoord.y;
		}

		float SMAASearchYDown( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 1.0, 0.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord -= vec2( 0.0, 2.0 ) * resolution; // WebGL port note: Changed sign
				if ( ! ( texcoord.y < end && e.r > 0.8281 && e.g == 0.0 ) ) break;
			}

			texcoord.y += 0.25 * resolution.y; // WebGL port note: Changed sign
			texcoord.y += resolution.y; // WebGL port note: Changed sign
			texcoord.y += 2.0 * resolution.y; // WebGL port note: Changed sign
			texcoord.y -= resolution.y * SMAASearchLength( searchTex, e.gr, 0.5, 0.5 ); // WebGL port note: Changed sign

			return texcoord.y;
		}

		vec2 SMAAArea( sampler2D areaTex, vec2 dist, float e1, float e2, float offset ) {
			// Rounding prevents precision errors of bilinear filtering:
			vec2 texcoord = float( SMAA_AREATEX_MAX_DISTANCE ) * round( 4.0 * vec2( e1, e2 ) ) + dist;

			// We do a scale and bias for mapping to texel space:
			texcoord = SMAA_AREATEX_PIXEL_SIZE * texcoord + ( 0.5 * SMAA_AREATEX_PIXEL_SIZE );

			// Move to proper place, according to the subpixel offset:
			texcoord.y += SMAA_AREATEX_SUBTEX_SIZE * offset;

			return texture2D( areaTex, texcoord, 0.0 ).rg;
		}

		vec4 SMAABlendingWeightCalculationPS( vec2 texcoord, vec2 pixcoord, vec4 offset[ 3 ], sampler2D edgesTex, sampler2D areaTex, sampler2D searchTex, ivec4 subsampleIndices ) {
			vec4 weights = vec4( 0.0, 0.0, 0.0, 0.0 );

			vec2 e = texture2D( edgesTex, texcoord ).rg;

			if ( e.g > 0.0 ) { // Edge at north
				vec2 d;

				// Find the distance to the left:
				vec2 coords;
				coords.x = SMAASearchXLeft( edgesTex, searchTex, offset[ 0 ].xy, offset[ 2 ].x );
				coords.y = offset[ 1 ].y; // offset[1].y = texcoord.y - 0.25 * resolution.y (@CROSSING_OFFSET)
				d.x = coords.x;

				// Now fetch the left crossing edges, two at a time using bilinear
				// filtering. Sampling at -0.25 (see @CROSSING_OFFSET) enables to
				// discern what value each edge has:
				float e1 = texture2D( edgesTex, coords, 0.0 ).r;

				// Find the distance to the right:
				coords.x = SMAASearchXRight( edgesTex, searchTex, offset[ 0 ].zw, offset[ 2 ].y );
				d.y = coords.x;

				// We want the distances to be in pixel units (doing this here allow to
				// better interleave arithmetic and memory accesses):
				d = d / resolution.x - pixcoord.x;

				// SMAAArea below needs a sqrt, as the areas texture is compressed
				// quadratically:
				vec2 sqrt_d = sqrt( abs( d ) );

				// Fetch the right crossing edges:
				coords.y -= 1.0 * resolution.y; // WebGL port note: Added
				float e2 = SMAASampleLevelZeroOffset( edgesTex, coords, ivec2( 1, 0 ) ).r;

				// Ok, we know how this pattern looks like, now it is time for getting
				// the actual area:
				weights.rg = SMAAArea( areaTex, sqrt_d, e1, e2, float( subsampleIndices.y ) );
			}

			if ( e.r > 0.0 ) { // Edge at west
				vec2 d;

				// Find the distance to the top:
				vec2 coords;

				coords.y = SMAASearchYUp( edgesTex, searchTex, offset[ 1 ].xy, offset[ 2 ].z );
				coords.x = offset[ 0 ].x; // offset[1].x = texcoord.x - 0.25 * resolution.x;
				d.x = coords.y;

				// Fetch the top crossing edges:
				float e1 = texture2D( edgesTex, coords, 0.0 ).g;

				// Find the distance to the bottom:
				coords.y = SMAASearchYDown( edgesTex, searchTex, offset[ 1 ].zw, offset[ 2 ].w );
				d.y = coords.y;

				// We want the distances to be in pixel units:
				d = d / resolution.y - pixcoord.y;

				// SMAAArea below needs a sqrt, as the areas texture is compressed
				// quadratically:
				vec2 sqrt_d = sqrt( abs( d ) );

				// Fetch the bottom crossing edges:
				coords.y -= 1.0 * resolution.y; // WebGL port note: Added
				float e2 = SMAASampleLevelZeroOffset( edgesTex, coords, ivec2( 0, 1 ) ).g;

				// Get the area for this direction:
				weights.ba = SMAAArea( areaTex, sqrt_d, e1, e2, float( subsampleIndices.x ) );
			}

			return weights;
		}

		void main() {

			gl_FragColor = SMAABlendingWeightCalculationPS( vUv, vPixcoord, vOffset, tDiffuse, tArea, tSearch, ivec4( 0.0 ) );

		}`},ta={name:"SMAABlendShader",uniforms:{tDiffuse:{value:null},tColor:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 2 ];

		void SMAANeighborhoodBlendingVS( vec2 texcoord ) {
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -1.0, 0.0, 0.0, 1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4( 1.0, 0.0, 0.0, -1.0 ); // WebGL port note: Changed sign in W component
		}

		void main() {

			vUv = uv;

			SMAANeighborhoodBlendingVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;
		uniform sampler2D tColor;
		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 2 ];

		vec4 SMAANeighborhoodBlendingPS( vec2 texcoord, vec4 offset[ 2 ], sampler2D colorTex, sampler2D blendTex ) {
			// Fetch the blending weights for current pixel:
			vec4 a;
			a.xz = texture2D( blendTex, texcoord ).xz;
			a.y = texture2D( blendTex, offset[ 1 ].zw ).g;
			a.w = texture2D( blendTex, offset[ 1 ].xy ).a;

			// Is there any blending weight with a value greater than 0.0?
			if ( dot(a, vec4( 1.0, 1.0, 1.0, 1.0 )) < 1e-5 ) {
				return texture2D( colorTex, texcoord, 0.0 );
			} else {
				// Up to 4 lines can be crossing a pixel (one through each edge). We
				// favor blending by choosing the line with the maximum weight for each
				// direction:
				vec2 offset;
				offset.x = a.a > a.b ? a.a : -a.b; // left vs. right
				offset.y = a.g > a.r ? -a.g : a.r; // top vs. bottom // WebGL port note: Changed signs

				// Then we go in the direction that has the maximum weight:
				if ( abs( offset.x ) > abs( offset.y )) { // horizontal vs. vertical
					offset.y = 0.0;
				} else {
					offset.x = 0.0;
				}

				// Fetch the opposite color and lerp by hand:
				vec4 C = texture2D( colorTex, texcoord, 0.0 );
				texcoord += sign( offset ) * resolution;
				vec4 Cop = texture2D( colorTex, texcoord, 0.0 );
				float s = abs( offset.x ) > abs( offset.y ) ? abs( offset.x ) : abs( offset.y );

				// WebGL port note: Added gamma correction
				C.xyz = pow(C.xyz, vec3(2.2));
				Cop.xyz = pow(Cop.xyz, vec3(2.2));
				vec4 mixed = mix(C, Cop, s);
				mixed.xyz = pow(mixed.xyz, vec3(1.0 / 2.2));

				return mixed;
			}
		}

		void main() {

			gl_FragColor = SMAANeighborhoodBlendingPS( vUv, vOffset, tColor, tDiffuse );

		}`};var ea=class extends Je{constructor(t,e){super(),this.edgesRT=new fe(t,e,{depthBuffer:!1,type:Ve}),this.edgesRT.texture.name="SMAAPass.edges",this.weightsRT=new fe(t,e,{depthBuffer:!1,type:Ve}),this.weightsRT.texture.name="SMAAPass.weights";let i=this,s=new Image;s.src=this.getAreaTexture(),s.onload=function(){i.areaTexture.needsUpdate=!0},this.areaTexture=new Pe,this.areaTexture.name="SMAAPass.area",this.areaTexture.image=s,this.areaTexture.minFilter=je,this.areaTexture.generateMipmaps=!1,this.areaTexture.flipY=!1;let r=new Image;r.src=this.getSearchTexture(),r.onload=function(){i.searchTexture.needsUpdate=!0},this.searchTexture=new Pe,this.searchTexture.name="SMAAPass.search",this.searchTexture.image=r,this.searchTexture.magFilter=Ee,this.searchTexture.minFilter=Ee,this.searchTexture.generateMipmaps=!1,this.searchTexture.flipY=!1,this.uniformsEdges=wn.clone(mr.uniforms),this.uniformsEdges.resolution.value.set(1/t,1/e),this.materialEdges=new te({defines:Object.assign({},mr.defines),uniforms:this.uniformsEdges,vertexShader:mr.vertexShader,fragmentShader:mr.fragmentShader}),this.uniformsWeights=wn.clone(gr.uniforms),this.uniformsWeights.resolution.value.set(1/t,1/e),this.uniformsWeights.tDiffuse.value=this.edgesRT.texture,this.uniformsWeights.tArea.value=this.areaTexture,this.uniformsWeights.tSearch.value=this.searchTexture,this.materialWeights=new te({defines:Object.assign({},gr.defines),uniforms:this.uniformsWeights,vertexShader:gr.vertexShader,fragmentShader:gr.fragmentShader}),this.uniformsBlend=wn.clone(ta.uniforms),this.uniformsBlend.resolution.value.set(1/t,1/e),this.uniformsBlend.tDiffuse.value=this.weightsRT.texture,this.materialBlend=new te({uniforms:this.uniformsBlend,vertexShader:ta.vertexShader,fragmentShader:ta.fragmentShader}),this.fsQuad=new oi(null)}render(t,e,i){this.uniformsEdges.tDiffuse.value=i.texture,this.fsQuad.material=this.materialEdges,t.setRenderTarget(this.edgesRT),this.clear&&t.clear(),this.fsQuad.render(t),this.fsQuad.material=this.materialWeights,t.setRenderTarget(this.weightsRT),this.clear&&t.clear(),this.fsQuad.render(t),this.uniformsBlend.tColor.value=i.texture,this.fsQuad.material=this.materialBlend,this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(e),this.clear&&t.clear(),this.fsQuad.render(t))}setSize(t,e){this.edgesRT.setSize(t,e),this.weightsRT.setSize(t,e),this.materialEdges.uniforms.resolution.value.set(1/t,1/e),this.materialWeights.uniforms.resolution.value.set(1/t,1/e),this.materialBlend.uniforms.resolution.value.set(1/t,1/e)}getAreaTexture(){return"data:image/png;base64,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"}getSearchTexture(){return"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEIAAAAhCAAAAABIXyLAAAAAOElEQVRIx2NgGAWjYBSMglEwEICREYRgFBZBqDCSLA2MGPUIVQETE9iNUAqLR5gIeoQKRgwXjwAAGn4AtaFeYLEAAAAASUVORK5CYII="}dispose(){this.edgesRT.dispose(),this.weightsRT.dispose(),this.areaTexture.dispose(),this.searchTexture.dispose(),this.materialEdges.dispose(),this.materialWeights.dispose(),this.materialBlend.dispose(),this.fsQuad.dispose()}};var pd=["PLANIFICADOR","REFUTADOR","SALA DE MANDO","PRINCIPAL","EMISARIO","AUDITOR","SKILLS","CUERPO","ALTURA","DAVANTIS","GIO"],n_=[...pd].sort((n,t)=>t.length-n.length);function Di(n){if(!n)return"";let t=n.indexOf("-");return(t===-1?n:n.slice(0,t)).toLowerCase()}var ia={"davantis-mando-admin":"SALA DE MANDO","davantis-mando":"SALA DE MANDO","davantis-altura":"ALTURA",davantis:"DAVANTIS",gio:"GIO"},i_={"crm-cabina":"davantis","b1-adjudicador":"davantis","davantis-crm":"davantis","davantis-admin":"davantis"},na="(servicios)";function md(n,t){let e=t&&t.actores||[],i=new Map((n||[]).map(o=>[o.id,o])),s=[],r=0;for(let o of e){let a=String(o&&o.principal||"").toLowerCase();if(!a)continue;let c={principal:a,calls:Math.max(0,Number(o.calls)||0),sondeo:Math.max(0,Number(o.sondeo)||0),trabajo:Math.max(0,Number(o.trabajo)||0),errores:Math.max(0,Number(o.errors)||0)+Math.max(0,Number(o.denied)||0),tools:Math.max(0,Number(o.tools)||0),proyecto:String(o.project||"")},l=ia[a],f=l&&i.get(l);if(f){let _=f.llamadas||{calls:0,sondeo:0,trabajo:0,errores:0,tools:0,principales:[]};f.llamadas={calls:_.calls+c.calls,sondeo:_.sondeo+c.sondeo,trabajo:_.trabajo+c.trabajo,errores:_.errores+c.errores,tools:Math.max(_.tools,c.tools),principales:[..._.principales||[],a]};continue}let d=Di(a),h=i_[a]||"",m=ia[d]?d:"",x=h||m;x||r++,s.push({id:a,principal:a,tipo:"actor",persona:x||na,exacta:!!h,...c})}return s.sort((o,a)=>a.calls-o.calls||o.id.localeCompare(a.id)),{terminales:n||[],actores:s,sinDeclarar:r}}function gd(n,t){let e=new Map,i=new Map,s=new Set((n||[]).map(r=>r.id));for(let[r,o]of Object.entries(ia))s.has(o)&&(e.set(r,{id:o,tipo:"terminal"}),Di(r)===r&&i.set(r,{id:o,tipo:"terminal"}));for(let r of t||[])e.set(r.principal,{id:r.id,tipo:"actor"});return{directo:e,porPersona:i}}function s_(){let n=new Map,t=new Map;for(let[e,i]of Object.entries(ia))n.set(e,{id:i,tipo:"terminal"}),Di(e)===e&&t.set(e,{id:i,tipo:"terminal"});return{directo:n,porPersona:t}}function xd(n,t){let e=String(n&&n.principal||"").toLowerCase();if(!e)return null;let i=t||s_(),s=i.directo.get(e);if(s)return{terminal:s.id,tipo:s.tipo,persona:Di(e),principal:e,exacta:!0};let r=Di(e),o=i.porPersona.get(r);return o?{terminal:o.id,tipo:o.tipo,persona:r,principal:e,exacta:!1}:null}function vd(n){let t=String(n&&n.kind||"").toLowerCase(),e=String(n&&n.outcome||"").toLowerCase(),i=Number(n&&n.ms);return{capa:t==="sondeo"?"sondeo":"trabajo",falla:e!==""&&e!=="ok",ms:Number.isFinite(i)&&i>=0?i:0,tool:String(n&&n.tool||"")}}var r_=["git-commit","sdd"],xr="libro mayor",vr="(sin atribuir)";function _d(n){let t=String(n&&n.topic||""),e=String(n&&n.domain||"");for(let s of r_)if(e===s||t===s||t.startsWith(s+"/"))return{clave:xr,tipo:"libro"};let i=Di(n&&n.author);return i?{clave:i,tipo:"persona"}:{clave:vr,tipo:"sin"}}function yd(n,t){let e=i=>i===vr?2:i===xr?1:0;return[...n].sort((i,s)=>e(i)-e(s)||(t[s]||0)-(t[i]||0)||String(i).localeCompare(String(s)))}var o_=n=>`${n.topic||""} ${n.gist||""}`.toUpperCase();function ud(n){for(let t of n_)if(n.includes(t))return t;return null}var dd=/([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-]{2,26})\s*(?:→|->)\s*\*{0,2}([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-·]{2,40})/g;function Md(n){let t=n&&n.neurons||[],e=new Map,i=new Map,s=0;for(let a of t){let c=o_(a),l=Di(a.author);l||s++;let f=(a.gist||"").toUpperCase().replace(/^[^A-ZÁÉÍÓÚÑ]+/,"");for(let h of pd){if(!c.includes(h))continue;let m=e.get(h);m||(m={id:h,notas:0,firmadas:0,calor:0,autores:new Map,firmas:new Map},e.set(h,m)),m.notas++,m.calor+=typeof a.heat=="number"&&a.heat>0?a.heat:0,f.startsWith(h)&&m.firmadas++,l&&(m.autores.set(l,(m.autores.get(l)||0)+1),f.startsWith(h)&&m.firmas.set(l,(m.firmas.get(l)||0)+1))}if(!c.includes("\u2192")&&!c.includes("->"))continue;dd.lastIndex=0;let d;for(;(d=dd.exec(c))!==null;){let h=ud(d[1]),m=ud(d[2]);if(!h||!m||h===m)continue;let x=h+">"+m;i.set(x,(i.get(x)||0)+1)}}let r=[...e.values()].map(a=>({id:a.id,notas:a.notas,firmas:a.firmadas,calor:a.calor,persona:fd(a.firmas)||fd(a.autores),autores:[...a.autores.entries()].sort((c,l)=>l[1]-c[1]).map(([c,l])=>({persona:c,notas:l}))})).sort((a,c)=>c.notas-a.notas||a.id.localeCompare(c.id)),o=[...i.entries()].map(([a,c])=>{let[l,f]=a.split(">");return{de:l,a:f,veces:c}}).sort((a,c)=>c.veces-a.veces||a.de.localeCompare(c.de)||a.a.localeCompare(c.a));return{terminales:r,despachos:o,sinAutor:s}}function fd(n){let t="",e=-1;for(let i of[...n.keys()].sort()){let s=n.get(i);s>e&&(e=s,t=i)}return t}function bd(n,t){let e=new Map,i=(s,r)=>{e.has(s)||e.set(s,[]),e.get(s).push(r)};for(let s of n||[])i(s.persona||"(sin autor)",{...s,tipo:"terminal"});for(let s of t||[])i(s.persona||na,s);return[...e.entries()].map(([s,r])=>({persona:s,nodos:r,notas:r.reduce((o,a)=>o+(a.tipo==="terminal"?a.notas:0),0),llamadas:r.reduce((o,a)=>o+(a.tipo==="actor"?a.calls:a.llamadas?a.llamadas.calls:0),0)})).sort((s,r)=>s.persona===na?1:r.persona===na?-1:r.notas-s.notas||s.persona.localeCompare(r.persona))}var ra=(n,t)=>[n[0]+t[0],n[1]+t[1],n[2]+t[2]],Ps=(n,t)=>[n[0]*t,n[1]*t,n[2]*t],sa=n=>{let t=Math.hypot(n[0],n[1],n[2])||1;return[n[0]/t,n[1]/t,n[2]/t]},Sd=(n,t)=>[n[1]*t[2]-n[2]*t[1],n[2]*t[0]-n[0]*t[2],n[0]*t[1]-n[1]*t[0]],a_=n=>()=>(n=n*1664525+1013904223>>>0)/4294967296;function c_(n,t,e){let i=Math.abs(n[1])>.9?[1,0,0]:[0,1,0],s=sa(Sd(n,i)),r=sa(Sd(n,s)),o=e()*Math.PI*2,a=ra(Ps(s,Math.cos(o)),Ps(r,Math.sin(o)));return sa(ra(Ps(n,Math.cos(t)),Ps(a,Math.sin(t))))}function l_(n){let t=Math.max(0,Number(n&&n.notas)||0),e=n&&n.centro||[0,0,0],i=Math.max(.001,Number(n&&n.escala)||1),s=Math.max(16,Number(n&&n.tope)||2e3),r=Math.max(1.4,Math.sqrt(t)*.42)*i,o=Math.max(4,Math.min(9,Math.round(Math.log(t+1)*1.7))),a=t>150?6:t>55?5:4,c=r*(t>120?2.4:2.9),l=Math.max(.28,r*.3),f=[],d=r,h=a_((Number(n&&n.semilla)||1)>>>0);function m(_,u,p,b,y,A){if(y>=a||b<.035*i||f.length>=s)return;let R=h()<.28?3:2;for(let T=0;T<R;T++){if(f.length>=s)return;let E=c_(u,.38+h()*.42,h),C=p*(.66+h()*.16),N=ra(_,Ps(E,C)),g=b*.62,S=A+C;S>d&&(d=S),f.push({a:_,b:N,w0:b,w1:g,nivel:y,dist:S}),m(N,E,C,g,y+1,S)}}for(let _=0;_<o;_++){let u=_+.5,p=Math.acos(1-2*u/o),b=Math.PI*(1+Math.sqrt(5))*u,y=sa([Math.cos(b)*Math.sin(p),Math.cos(p),Math.sin(b)*Math.sin(p)]);m(ra(e,Ps(y,r*.85)),y,c,l,0,r*.85)}let x=r;for(let _ of f){let u=Math.hypot(_.b[0]-e[0],_.b[1]-e[1],_.b[2]-e[2]);u>x&&(x=u)}return{segs:f,alcance:x,alcanceRama:d,rSoma:r}}function wd(n,t){let e=t||{},i=Math.max(16,Number(e.topePorArbol)||2e3),s=[],r=0,o=7;for(let a of n||[]){let c=a.troncos||[];if(!c.length)continue;let l=Math.max(1,Number(a.radio)||1);c.forEach((f,d)=>{let h=d+.5,m=Math.acos(1-2*h/c.length),x=Math.PI*(1+Math.sqrt(5))*h,u=String(f.id||"").toLowerCase()===String(a.persona||"").toLowerCase()?l*.12:l*.62,p=[a.centro[0]+Math.cos(x)*Math.sin(m)*u,a.centro[1]+Math.cos(m)*u,a.centro[2]+Math.sin(x)*Math.sin(m)*u],b=l_({id:f.id,notas:f.notas,centro:p,escala:Number(e.escala)||1,tope:i,semilla:o+=977});r+=b.segs.length,s.push({id:f.id,persona:a.persona,notas:f.notas,centro:p,color:a.color,...b})})}return{troncos:s,total:r}}var Ad=[1,.6,.02];function h_(n){return 1+Math.min(1.6,Math.log10(1+Math.max(0,Number(n)||0))*.42)}function Ed(){let n=new Map,t=0,e=0;return{nacer(i,s,r){t++;let o=i;if(!Number.isInteger(o)||o<0)return e++,!1;let a=n.get(o);return a||(a=[],n.set(o,a)),a.length>=6&&a.shift(),a.push({t0:Number(r)||0,trabajo:(s&&s.capa)!=="sondeo",falla:!!(s&&s.falla),grosor:h_(s&&s.ms),exacta:!(s&&s.exacta===!1)}),!0},cuenta(){return{vistos:t,sinTronco:e}},vivos(i){let s=0;for(let r of n.values())for(let o of r)i-o.t0<=.85&&s++;return s},frentes(i,s,r){let o=n.get(Number(i));if(!o||!o.length)return[];for(;o.length&&s-o[0].t0>.85;)o.shift();let a=Math.max(.001,Number(r)||1),c=[];for(let l of o){let f=(s-l.t0)/.85;f<0||c.push({en:f*a,radio:.22*a*l.grosor,fuerza:(l.trabajo?1:.3)*(1-f*f)*(l.exacta?1:.65)*l.grosor,falla:l.falla})}return c},escribir(i,s){let r=s.alcances.length,o=new Float32Array(r),a=new Array(r),c=!1;for(let d=0;d<r;d++){let h=this.frentes(d,i,s.alcances[d]);if(a[d]=h.length?h:null,h.length){c=!0;for(let m of h)m.en<m.radio&&(o[d]=Math.max(o[d],m.fuerza*(1-m.en/m.radio)))}}let l=0,f=s.glow.length;if(!c)return s.glow.fill(0),s.warn.fill(0),{encendidas:0,flash:o};for(let d=0;d<f;d++){let h=a[s.tronco[d]];if(!h){s.glow[d]=0,s.warn[d]=0;continue}let m=s.dist[d],x=0,_=0;for(let u=0;u<h.length;u++){let p=h[u],b=m<p.en?p.en-m:m-p.en;if(b>=p.radio)continue;let y=p.fuerza*(1-b/p.radio);x+=y,p.falla&&(_+=y)}s.glow[d]=x,s.warn[d]=x>0?Math.min(1,_/x):0,x>.004&&l++}return{encendidas:l,flash:o}}}}var u_=[79,107,255],d_=[63,208,163],yr=[53,208,224],oa=[u_,d_,[160,107,255],yr,[232,178,74]],Rd=n=>{let t=oa[n%oa.length];return`rgb(${t[0]},${t[1]},${t[2]})`};var Xy=`rgb(${yr[0]},${yr[1]},${yr[2]})`,Td=n=>()=>(n=n*1664525+1013904223>>>0)/4294967296,f_=n=>{let t=2166136261;for(let e=0;e<n.length;e++)t=Math.imul(t^n.charCodeAt(e),16777619);return(t>>>0)%1e3/1e3*6.2832},ai=(n,t)=>[n[0]+t[0],n[1]+t[1],n[2]+t[2]],kn=(n,t)=>[n[0]*t,n[1]*t,n[2]*t],Mr=n=>{let t=Math.hypot(n[0],n[1],n[2])||1;return[n[0]/t,n[1]/t,n[2]/t]},Cd=(n,t)=>[n[1]*t[2]-n[2]*t[1],n[2]*t[0]-n[0]*t[2],n[0]*t[1]-n[1]*t[0]],_r=(n,t,e)=>Math.max(t,Math.min(e,n));function p_(n,t,e){let i=Math.abs(n[1])>.9?[1,0,0]:[0,1,0],s=Mr(Cd(n,i)),r=Mr(Cd(n,s)),o=e()*Math.PI*2,a=ai(kn(s,Math.cos(o)),kn(r,Math.sin(o)));return Mr(ai(kn(n,Math.cos(t)),kn(a,Math.sin(t))))}function Pd(n){let t=n.getContext("2d"),e={yaw:.2,pitch:-.24,dist:1180,f:2800,zoom:1},i=.4,s=40,r=2,o=0,a=0,c=null,l=[],f=[],d=new Map,h=[0,0,0],m=600,x=300,_=0,u=0,p=0,b=0,y=null,A=!1,R="rotar",T=0,E=0,C=!1,N=0,g=1,S=null,F=[],V=!matchMedia("(prefers-reduced-motion: reduce)").matches,q=.85,et=.22,B=6,it=0,Z=0;function gt(){let j=Math.min(devicePixelRatio||1,2);o=n.clientWidth,a=n.clientHeight,n.width=Math.round(o*j),n.height=Math.round(a*j),t.setTransform(j,0,0,j,0,0),J()}function _t(){let ct=20,ut=20,Rt=o-20,z=a-20,M=k=>{let nt=document.querySelector(k);if(!nt)return null;let Q=nt.getBoundingClientRect();return Q.width>1&&Q.height>1?Q:null},v=M("#hud .rail.l");v&&(ct=Math.max(ct,v.right+20));let P=M("#hud .rail.r");P&&(Rt=Math.min(Rt,P.left-20));let H=M("#hud .hdr");H&&(ut=Math.max(ut,H.bottom+20));let K=M("#hud .center");return K&&(z=Math.min(z,K.top-20)),Rt-ct<120||z-ut<120?{x0:0,y0:0,x1:o,y1:a}:{x0:ct,y0:ut,x1:Rt,y1:z}}let bt=.5,Gt=200,Wt=200;function J(){if(!o||!a)return;let j=_t();_=(j.x0+j.x1)/2,u=(j.y0+j.y1)/2,Gt=Math.max((j.x1-j.x0)/2*.96,60),Wt=Math.max((j.y1-j.y0)/2*.96,60);let ct=x*Math.cos(bt)+m*Math.sin(bt);e.dist=Math.max(240,m+e.f*m/Gt,m+e.f*ct/Wt)}let st=()=>e.f/(e.dist/e.zoom);function Mt(j,ct){if(c=j,l=[],f=[],d=new Map,p=0,b=0,e.zoom=1,S=null,!ct.length)return;let ut=X=>88+36*Math.sqrt(X.nodos.length),Rt=92,z=0,M=ct.map((X,ht)=>(ht>0&&(z+=ut(ct[ht-1])+Rt+ut(X)),[z,0,0])),v=M.length?(M[0][0]+M[M.length-1][0])/2:0;for(let X of M)X[0]-=v;g=1,ct.forEach((X,ht)=>{let tt=oa[ht%oa.length],lt=M[ht],At=ut(X);X.nodos.forEach((rt,ot)=>{let Lt=ot+.5,wt=Math.acos(1-2*Lt/X.nodos.length),Ft=Math.PI*(1+Math.sqrt(5))*Lt,L=rt.id.toLowerCase()===X.persona?At*.18:At,ft=rt.tipo==="actor",Y=!ft&&Number.isFinite(rt.calor)?rt.calor:0;Y>g&&(g=Y);let $={id:rt.id,tipo:ft?"actor":"terminal",notas:ft?0:rt.notas,calor:Y,firmas:rt.firmas||0,persona:X.persona,col:tt,llamadas:ft?{calls:rt.calls,sondeo:rt.sondeo,trabajo:rt.trabajo,tools:rt.tools,proyecto:rt.proyecto}:rt.llamadas||null,exacta:ft?rt.exacta!==!1:!0,r:ft?Math.max(3.4,Math.log10(1+(rt.calls||0))*3.4):Math.max(4.2,Math.sqrt(rt.notas)*1.15),fase:f_(rt.id),seg:[],finNivel:[0],alcance:0,alcanceRama:1,pulsos:[],pos:[lt[0]+Math.cos(Ft)*Math.sin(wt)*L,lt[1]+Math.cos(wt)*L*.78,lt[2]+Math.sin(Ft)*Math.sin(wt)*L]};f.push($),d.set(rt.id,$)})});let P=7;for(let X of f){let ht=Td(P+=977);if(X.tipo==="actor"){Nt(X);continue}let tt=Math.max(4,Math.min(7,Math.round(Math.log(X.notas+1)*1.6)));X.profBase=X.notas>150?5:X.notas>55?4:3;let lt=X.r*(X.notas>120?2.5:2.9);for(let rt=0;rt<tt;rt++){let ot=rt+.5,Lt=Math.acos(1-2*ot/tt),wt=Math.PI*(1+Math.sqrt(5))*ot,Ft=Mr([Math.cos(wt)*Math.sin(Lt),Math.cos(Lt),Math.sin(wt)*Math.sin(Lt)]);xt(ai(X.pos,kn(Ft,X.r*.85)),Ft,lt,Math.max(1.5,X.r*.3),X.profBase+r,0,X,ht,X.r*.85)}X.seg.sort((rt,ot)=>rt.nivel-ot.nivel);let At=X.profBase+r;X.finNivel=new Array(At+1).fill(0);for(let rt of X.seg)for(let ot=rt.nivel;ot<=At;ot++)X.finNivel[ot]++;for(let rt of X.seg){let ot=Math.hypot(rt.b[0]-X.pos[0],rt.b[1]-X.pos[1],rt.b[2]-X.pos[2]);ot>X.alcance&&(X.alcance=ot)}}for(let X of c.despachos){let ht=d.get(X.de),tt=d.get(X.a);if(!ht||!tt)continue;let lt=Td(X.de.length*131+X.a.length*17+X.veces),At=kn(ai(ht.pos,tt.pos),.5),rt=ai(At,[(lt()-.5)*150,-70-X.veces*4.5-lt()*60,(lt()-.5)*150]),ot=[];for(let Lt=0;Lt<=26;Lt++){let wt=Lt/26,Ft=1-wt;ot.push([Ft*Ft*ht.pos[0]+2*Ft*wt*rt[0]+wt*wt*tt.pos[0],Ft*Ft*ht.pos[1]+2*Ft*wt*rt[1]+wt*wt*tt.pos[1],Ft*Ft*ht.pos[2]+2*Ft*wt*rt[2]+wt*wt*tt.pos[2]])}l.push({de:X.de,a:X.a,veces:X.veces,P:ot,cruza:ht.persona!==tt.persona,Q:new Array(ot.length)})}let H=[1e9,1e9,1e9],K=[-1e9,-1e9,-1e9];for(let X of f)for(let ht=0;ht<3;ht++)X.pos[ht]<H[ht]&&(H[ht]=X.pos[ht]),X.pos[ht]>K[ht]&&(K[ht]=X.pos[ht]);h=[(H[0]+K[0])/2,(H[1]+K[1])/2,(H[2]+K[2])/2];let k=0,nt=0,Q=46;for(let X of f){let ht=Math.hypot(X.pos[0]-h[0],X.pos[2]-h[2]);ht+Q>k&&(k=ht+Q);let tt=Math.abs(X.pos[1]-h[1]);tt+Q>nt&&(nt=tt+Q)}m=Math.max(120,k),x=Math.max(80,nt),J()}function xt(j,ct,ut,Rt,z,M,v,P,H){if(z<=0||Rt<.06||v.seg.length>9e3)return;let K=P()<.28?3:2;for(let k=0;k<K;k++){let nt=p_(ct,.38+P()*.42,P),Q=ut*(.66+P()*.16),X=ai(j,kn(nt,Q)),ht=H+Q;ht>v.alcanceRama&&(v.alcanceRama=ht),v.seg.push({a:j,b:X,w:Rt,nivel:M,dist:ht}),xt(X,nt,Q,Rt*.6,z-1,M+1,v,P,ht)}}function Nt(j){let ct=Math.max(2,Math.min(12,j.llamadas&&j.llamadas.tools||2)),ut=j.r*2.4;for(let Rt=0;Rt<ct;Rt++){let z=Rt+.5,M=Math.acos(1-2*z/ct),v=Math.PI*(1+Math.sqrt(5))*z,P=Mr([Math.cos(v)*Math.sin(M),Math.cos(M),Math.sin(v)*Math.sin(M)]),H=ai(j.pos,kn(P,j.r*.9)),K=ai(j.pos,kn(P,j.r*.9+ut));j.seg.push({a:H,b:K,w:Math.max(.9,j.r*.22),nivel:0,dist:j.r*.9+ut})}j.alcanceRama=j.r*.9+ut,j.alcance=j.alcanceRama,j.profBase=0,j.finNivel=new Array(r+1).fill(j.seg.length)}function It(j){it++;let ct=j&&j.terminal?d.get(j.terminal):null;return ct?(ct.pulsos.length>=B&&ct.pulsos.shift(),ct.pulsos.push({t0:N,capa:j.capa==="sondeo"?"sondeo":"trabajo",falla:!!j.falla,grosor:1+Math.min(1.6,Math.log10(1+Math.max(0,Number(j.ms)||0))*.42),exacta:j.exacta!==!1}),!0):(Z++,!1)}function kt(j){let ct=j[0]-h[0],ut=j[1]-h[1],Rt=j[2]-h[2],z=Math.cos(e.yaw),M=Math.sin(e.yaw),v=ct*z-Rt*M,P=ct*M+Rt*z,H=ut,K=Math.cos(e.pitch),k=Math.sin(e.pitch),nt=H*K-P*k,X=H*k+P*K+e.dist/e.zoom,ht=e.f/Math.max(X,40);return{x:_+p+v*ht,y:u+b+nt*ht,s:ht,z:X}}function Zt(){if(t.clearRect(0,0,o,a),!f.length){t.fillStyle="rgba(154,157,171,.7)",t.font="13px 'JetBrains Mono', ui-monospace, monospace",t.textAlign="center",t.fillText("todav\xEDa no hay despachos entre terminales en esta memoria",_||o/2,u||a/2);return}F.length=0;let j=e.dist/e.zoom-m,ct=Math.max(2*m,1),ut=z=>_r((z-j)/ct,0,1),Rt=_r(Math.round(Math.log2(Math.max(e.zoom,1))),0,r);for(let z of f){let M=kt(z.pos),v=z.alcance*M.s+40;if(M.x+v<0||M.x-v>o||M.y+v<0||M.y-v>a){z._sx=null;continue}z._P=M;let P=z.pulsos;for(;P.length&&N-P[0].t0>q;)P.shift();let H=[];for(let k=0;k<P.length;k++){let nt=P[k],Q=(N-nt.t0)/q;H.push({en:Q*z.alcanceRama,radio:et*z.alcanceRama,fuerza:(nt.capa==="trabajo"?1:.3)*(1-Q*Q)*(nt.exacta?1:.65),falla:nt.falla,grosor:nt.grosor})}z._flash=0;for(let k of H)k.en<k.radio&&(z._flash=Math.max(z._flash,k.fuerza*(1-k.en/k.radio)));F.push({z:M.z,t:2,nd:z,P:M});let K=z.finNivel[Math.min(z.finNivel.length-1,z.profBase+Rt)];for(let k=0;k<K;k++){let nt=z.seg[k],Q=kt(nt.a),X=kt(nt.b);if(Math.abs(Q.x-X.x)+Math.abs(Q.y-X.y)<.8)continue;let ht=0,tt=1,lt=!1;for(let At=0;At<H.length;At++){let rt=H[At],ot=Math.abs(nt.dist-rt.en)/rt.radio;if(ot>=1)continue;let Lt=(1-ot)*(1-ot);ht+=Lt*rt.fuerza,Lt>.35&&(tt=Math.max(tt,rt.grosor),lt=lt||rt.falla)}F.push({z:(Q.z+X.z)/2,t:0,A:Q,B:X,w:nt.w,col:z.col,nodo:z.id,carga:ht>.01?Math.min(1.6,ht):0,gordo:tt,falla:lt})}}for(let z of l){for(let M=0;M<z.P.length;M++)z.Q[M]=kt(z.P[M]);for(let M=0;M<z.Q.length-1;M++){let v=z.Q[M],P=z.Q[M+1];F.push({z:(v.z+P.z)/2,t:1,A:v,B:P,ax:z,ult:M===z.Q.length-2})}}F.sort((z,M)=>M.z-z.z);for(let z of F)if(z.t===0){let M=y&&y!==z.nodo,v=1-.78*ut(z.A.z),P=z.col;if(t.lineCap="round",t.strokeStyle=`rgba(${P[0]},${P[1]},${P[2]},${((M?.055:.3)*v).toFixed(3)})`,t.lineWidth=Math.max(.35,z.w*z.A.s*1.15),t.beginPath(),t.moveTo(z.A.x,z.A.y),t.lineTo(z.B.x,z.B.y),t.stroke(),z.carga>0&&!M){let H=Math.min(1,z.carga),K=Math.round(P[0]+(255-P[0])*H*.9),k=Math.round(P[1]+(255-P[1])*H*.9),nt=Math.round(P[2]+(255-P[2])*H*.75),Q=z.falla?[245,196,81]:[K,k,nt],X=Math.max(.6,z.w*z.A.s*1.15),ht=t.globalCompositeOperation;t.globalCompositeOperation="lighter",t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${(.2*H*v).toFixed(3)})`,t.lineWidth=Math.max(3.2,X*z.gordo*5.5),t.beginPath(),t.moveTo(z.A.x,z.A.y),t.lineTo(z.B.x,z.B.y),t.stroke(),t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${(Math.min(1,z.carga)*v).toFixed(3)})`,t.lineWidth=Math.max(1.5,X*z.gordo*2.1),t.beginPath(),t.moveTo(z.A.x,z.A.y),t.lineTo(z.B.x,z.B.y),t.stroke(),t.globalCompositeOperation=ht}}else if(z.t===1){let M=y&&(y===z.ax.de||y===z.ax.a),v=y&&!M,P=1-.6*ut(z.A.z),H=z.ax.cruza?yr:[138,153,255],K=M?.62:v?.04:.27;if(t.strokeStyle=`rgba(${H[0]},${H[1]},${H[2]},${(K*P).toFixed(3)})`,t.lineWidth=Math.max(.5,(.7+z.ax.veces*.16)*z.A.s*(M?1.5:1)),t.beginPath(),t.moveTo(z.A.x,z.A.y),t.lineTo(z.B.x,z.B.y),t.stroke(),z.ult){let k=Math.atan2(z.B.y-z.A.y,z.B.x-z.A.x),nt=Math.max(4,7*z.B.s*(M?1.5:1));t.fillStyle=`rgba(${H[0]},${H[1]},${H[2]},${((K+.14)*P).toFixed(3)})`,t.beginPath(),t.moveTo(z.B.x,z.B.y),t.lineTo(z.B.x-Math.cos(k-.42)*nt,z.B.y-Math.sin(k-.42)*nt),t.lineTo(z.B.x-Math.cos(k+.42)*nt,z.B.y-Math.sin(k+.42)*nt),t.closePath(),t.fill()}}else{let M=z.nd,v=z.P,P=y&&y!==M.id,H=Math.log(1+M.calor)/Math.log(1+g),K=1+.17*H*Math.sin(N*1.9+M.fase),k=M._flash||0,nt=Math.max(2.2,M.r*v.s*1.5)*K*(1+.9*k),Q=M.col,X=nt+Math.min(nt*1.6,38),ht=t.createRadialGradient(v.x,v.y,0,v.x,v.y,X);if(ht.addColorStop(0,`rgba(${Q[0]},${Q[1]},${Q[2]},${P?.1:.22+.14*H*(K-1)/.17})`),ht.addColorStop(1,`rgba(${Q[0]},${Q[1]},${Q[2]},0)`),t.fillStyle=ht,t.beginPath(),t.arc(v.x,v.y,X,0,6.2832),t.fill(),M.tipo==="actor"?(t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${P?.3:.95})`,t.lineWidth=Math.max(1.2,nt*.34),M.exacta===!1&&t.setLineDash([Math.max(2,nt*.5),Math.max(2,nt*.42)]),t.beginPath(),t.arc(v.x,v.y,nt,0,6.2832),t.stroke(),t.setLineDash([]),k>.01&&(t.fillStyle=`rgba(255,255,255,${(.75*k).toFixed(3)})`,t.beginPath(),t.arc(v.x,v.y,nt*.52*k,0,6.2832),t.fill())):(t.fillStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${P?.28:.95})`,t.beginPath(),t.arc(v.x,v.y,nt,0,6.2832),t.fill(),t.fillStyle=`rgba(255,255,255,${P?.06:Math.min(1,.22+.78*k).toFixed(3)})`,t.beginPath(),t.arc(v.x,v.y,nt*(.42+.18*k),0,6.2832),t.fill()),M._sx=v.x,M._sy=v.y,M._sr=nt,!P){let tt=M.tipo==="actor"?M.llamadas&&M.llamadas.calls>=1e3:M.notas>=60;t.fillStyle=y===M.id?"#ECEAE3":tt?"rgba(236,234,227,.62)":"rgba(236,234,227,.34)";let lt=M.tipo==="actor";t.font=`${Math.max(8,Math.min(lt?12:16,(lt?9:11)*v.s*1.3))}px 'JetBrains Mono', ui-monospace, monospace`,t.textAlign="center";let At=lt&&M.persona&&M.id.startsWith(M.persona+"-")?M.id.slice(M.persona.length+1):M.id;t.fillText(At.toLowerCase(),v.x,v.y+nt+(lt?11:14))}}}let Vt=j=>{let ct=n.getBoundingClientRect(),ut=j.clientX-ct.left,Rt=j.clientY-ct.top,z=null,M=1e9;for(let v of f){if(v._sx==null)continue;let P=Math.hypot(ut-v._sx,Rt-v._sy);P<Math.max(16,v._sr*1.9)&&P<M&&(M=P,z=v)}return z};n.addEventListener("pointerdown",j=>{A=!0,R=j.shiftKey||j.button===1?"pan":"rotar",T=j.clientX,E=j.clientY,S=null,I();try{n.setPointerCapture(j.pointerId)}catch{}}),n.addEventListener("pointerup",()=>{A=!1}),n.addEventListener("pointermove",j=>{if(A){R==="pan"?(p+=j.clientX-T,b+=j.clientY-E):(e.yaw+=(j.clientX-T)*.006,e.pitch=_r(e.pitch+(j.clientY-E)*.005,-1.2,1.2)),T=j.clientX,E=j.clientY;return}let ct=Vt(j),ut=ct?ct.id:null;ut!==y&&(y=ut,me&&me(ct,c,j.clientX,j.clientY))}),n.addEventListener("pointerleave",()=>{I()});function I(){y!==null&&(y=null,me&&me(null,c))}n.addEventListener("wheel",j=>{j.preventDefault();let ct=n.getBoundingClientRect(),ut=j.clientX-ct.left,Rt=j.clientY-ct.top,z=st();e.zoom=_r(e.zoom*(j.deltaY>0?1/1.12:1.12),i,s);let M=st()/z;p=ut-_-(ut-_-p)*M,b=Rt-u-(Rt-u-b)*M,S=null,I()},{passive:!1}),n.addEventListener("dblclick",j=>{let ct=Vt(j);if(!ct){S={zoom:1,panx:0,pany:0,t:0};return}let ut=Math.max(ct.alcance,ct.r*3),Rt=_r(.62*Math.min(Gt,Wt)*e.dist/(e.f*ut),1,s),z=e.zoom,M=p,v=b;e.zoom=Rt,p=0,b=0;let P=kt(ct.pos),H={zoom:Rt,panx:_-P.x,pany:u-P.y,t:0};e.zoom=z,p=M,b=v,S=H});let me=null;return addEventListener("resize",()=>{C&&gt()}),{cargar(j,ct){Mt(j,ct)},activar(j){C=j,j&&gt()},activa(){return C},frame(j){if(C){if(j&&V&&(N+=1/60,A||y||e.zoom>1.35||p||b||(e.yaw+=.0018)),S){S.t=Math.min(1,S.t+.075);let ct=1-Math.pow(1-S.t,3);e.zoom+=(S.zoom-e.zoom)*ct*.5,p+=(S.panx-p)*ct*.5,b+=(S.pany-b)*ct*.5,S.t>=1&&(e.zoom=S.zoom,p=S.panx,b=S.pany,S=null)}Zt()}},onFoco(j){me=j},pulsar:It,cuentaPulsos(){return{vistos:it,sinNeurona:Z}}}}function Id(n){return n>2e3?55:n>500?90:n>180?150:230}function sh(n,t,e){return e?t<=0?0:Math.min(Id(n),4+t*2):Id(n)}var m_=700,g_=.7*.7,Ie=0,Vn,ci,Ni,Oi,Fi,gn,Bi,zi,ki,Hi,th=0;function Ud(n){n<=Ie||(Ie=Math.max(n,Ie*2,1024),Vn=new Int32Array(Ie*8),ci=new Int32Array(Ie),Ni=new Float64Array(Ie),Oi=new Float64Array(Ie),Fi=new Float64Array(Ie),gn=new Int32Array(Ie),Bi=new Float64Array(Ie),zi=new Float64Array(Ie),ki=new Float64Array(Ie),Hi=new Float64Array(Ie))}function eh(n,t,e,i){th>=Ie&&x_();let s=th++;return Vn.fill(-1,s*8,s*8+8),ci[s]=0,Ni[s]=Oi[s]=Fi[s]=0,gn[s]=-1,Bi[s]=i,zi[s]=n,ki[s]=t,Hi[s]=e,s}function x_(){let n={chi:Vn,cnt:ci,cx:Ni,cy:Oi,cz:Fi,bod:gn,h:Bi,ox:zi,oy:ki,oz:Hi,n:Ie};Ie=n.n,Ud(n.n*2||1024),Vn.set(n.chi),ci.set(n.cnt),Ni.set(n.cx),Oi.set(n.cy),Fi.set(n.cz),gn.set(n.bod),Bi.set(n.h),zi.set(n.ox),ki.set(n.oy),Hi.set(n.oz)}function Ld(n,t,e,i){return(t>zi[n]?1:0)|(e>ki[n]?2:0)|(i>Hi[n]?4:0)}function v_(n,t,e,i,s){let r=n;for(let o=0;o<64;o++){if(ci[r]++,Ni[r]+=e,Oi[r]+=i,Fi[r]+=s,ci[r]===1){gn[r]=t;return}if(gn[r]>=0){let f=gn[r];gn[r]=-1;let d=Ge[f].x,h=Ge[f].y,m=Ge[f].z,x=Ld(r,d,h,m),_=Bi[r]*.5,u=Vn[r*8+x];u<0&&(u=eh(zi[r]+(x&1?_:-_),ki[r]+(x&2?_:-_),Hi[r]+(x&4?_:-_),_),Vn[r*8+x]=u),ci[u]++,Ni[u]+=d,Oi[u]+=h,Fi[u]+=m,gn[u]=f}let a=Ld(r,e,i,s),c=Bi[r]*.5,l=Vn[r*8+a];l<0&&(l=eh(zi[r]+(a&1?c:-c),ki[r]+(a&2?c:-c),Hi[r]+(a&4?c:-c),c),Vn[r*8+a]=l),r=l}}var aa=new Int32Array(16384);function __(n,t,e,i,s,r,o,a){let c=0;aa[c++]=n;let l=0,f=0,d=0;for(;c>0;){let h=aa[--c],m=ci[h];if(m===0)continue;let x=Ni[h]/m,_=Oi[h]/m,u=Fi[h]/m,p=e-x,b=i-_,y=s-u,A=p*p+b*b+y*y,R=gn[h]>=0;if(R&&gn[h]===t)continue;let T=Bi[h]*2;if(R||T*T<g_*A){if(A>r||A<.001)continue;let E=o*m/A,C=Math.sqrt(A);l+=p/C*E,f+=b/C*E,d+=y/C*E}else for(let E=0;E<8;E++){let C=Vn[h*8+E];C>=0&&c<aa.length&&(aa[c++]=C)}}a[0]=l,a[1]=f,a[2]=d}var ca=new Float64Array(3),Ge=[],Nd=[],nh=()=>({rx:118,ry:94,rz:87}),Ui=0,Od=0,Fd=0,Bd=0,ih=!1,y_=.09,M_=.0042,b_=.86,Dd=.055,S_=256,Is=0,Hn=0,$l=0;function w_(n,t,e,i){if(n.gr){let r=n.x-(n.gx||0),o=n.y-(n.gy||0),a=n.z-(n.gz||0),c=r*r+o*o+a*a;if(c>n.gr*n.gr){let l=n.gr/Math.sqrt(c);n.x=(n.gx||0)+r*l,n.y=(n.gy||0)+o*l,n.z=(n.gz||0)+a*l,n.vx*=.4,n.vy*=.4,n.vz*=.4}return}let s=n.x*n.x/(t*t)+n.y*n.y/(e*e)+n.z*n.z/(i*i);if(s>1){let r=1/Math.sqrt(s);n.x*=r,n.y*=r,n.z*=r,n.vx*=.4,n.vy*=.4,n.vz*=.4}}function zd(n,t,e,i){Ge=n,Nd=t,nh=e;let s=Ge.length;if(!s){Ui=0;return}let r=nh().rx,o=r*.85;Od=o*o,Fd=r*r*.06,Bd=r*.044,ih=s>=m_,ih&&Ud(Math.max(1024,s*4)),Ui=i,Is=0,Hn=0}function rh(){return Ui}function kd(n){if(Ui<=0)return{trabajo:!1,termino:!1};let t=performance.now();do if(A_(t,n))Ui--;else break;while(Ui>0&&performance.now()-t<n);return{trabajo:!0,termino:Ui===0}}function A_(n,t){let e=Ge.length;if(!e)return!0;let i=Od,s=Fd,r=Bd,o=y_,a=M_,c=b_,l=ih;if(Is===0){if(l){let d=1;for(let h of Ge){let m=Math.max(Math.abs(h.x),Math.abs(h.y),Math.abs(h.z));m>d&&(d=m)}th=0,$l=eh(0,0,0,d*1.05);for(let h=0;h<e;h++){let m=Ge[h];v_($l,h,m.x,m.y,m.z)}}Is=1,Hn=0}if(Is===1){if(l)for(;Hn<e;){let d=Math.min(e,Hn+S_);for(let h=Hn;h<d;h++){let m=Ge[h];__($l,h,m.x,m.y,m.z,i,s,ca);let x=m.gx!==void 0?Dd:a,_=m.gx||0,u=m.gy||0,p=m.gz||0;m.vx+=ca[0]*.5-(m.x-_)*x,m.vy+=ca[1]*.5-(m.y-u)*x,m.vz+=ca[2]*.5-(m.z-p)*x}if(Hn=d,Hn<e&&performance.now()-n>=t)return!1}else{for(let d=0;d<e;d++){let h=Ge[d],m=0,x=0,_=0;for(let A=d+1;A<e;A++){let R=Ge[A],T=h.x-R.x,E=h.y-R.y,C=h.z-R.z,N=T*T+E*E+C*C;if(N>i||N<.001)continue;let g=s/N,S=Math.sqrt(N),F=T/S,V=E/S,q=C/S;m+=F*g,x+=V*g,_+=q*g,R.vx-=F*g*.5,R.vy-=V*g*.5,R.vz-=q*g*.5}let u=h.gx!==void 0?Dd:a,p=h.gx||0,b=h.gy||0,y=h.gz||0;h.vx+=m*.5-(h.x-p)*u,h.vy+=x*.5-(h.y-b)*u,h.vz+=_*.5-(h.z-y)*u}Hn=e}Is=2}for(let d of Nd){let h=Ge[d.a],m=Ge[d.b],x=m.x-h.x,_=m.y-h.y,u=m.z-h.z,p=Math.hypot(x,_,u)||1,b=(p-r)*o,y=x/p,A=_/p,R=u/p;h.vx+=y*b,h.vy+=A*b,h.vz+=R*b,m.vx-=y*b,m.vy-=A*b,m.vz-=R*b}let f=nh();for(let d of Ge){d.vx*=c,d.vy*=c,d.vz*=c;let h=d.vx*d.vx+d.vy*d.vy+d.vz*d.vz;if(h>1600){let m=40/Math.sqrt(h);d.vx*=m,d.vy*=m,d.vz*=m}d.x+=d.vx,d.y+=d.vy,d.z+=d.vz,w_(d,f.rx,f.ry,f.rz)}return Is=0,Hn=0,!0}var ya=["#2dd4bf","#a78bfa","#fbbf24","#4ade80","#38bdf8","#f472b6","#fb923c","#f87171","#a3e635","#22d3ee","#e879f9","#facc15"];var fh=["#7f9cc9","#43e08b","#31c9ff","#f5c451"],Jd=fh[0],Vi={CALLS:"#38bdf8",IMPORTS:"#a78bfa",CONTAINS:"#5b6b86"},ph="#8a99ff",mh="#2dd4bf",Qd=n=>n&&n.kind&&Vi[n.kind]||Jd,Pr=new Map,vn=[],$d=n=>Pr.get(n)||"#64748b",E_="#98a2b3",T_="#78839a";function Hd(n){return n._grupoTipo==="libro"?"libro mayor \xB7 lo escribe el motor":n._grupoTipo==="sin"?"sin autor \xB7 anterior a la columna":"de "+(n._grupo||"?")}function C_(n,t){return n===xr?E_:n===vr?T_:ya[t%ya.length]}function tf(n){let t=2166136261;for(let e=0;e<n.length;e++)t^=n.charCodeAt(e),t=Math.imul(t,16777619);return(t>>>0)%1e3/1e3}var R_=118,Ah=1,Wn=118,Ws=94,Xs=87;function ef(){Wn=R_*Ah,Ws=Wn,Xs=Wn}var gh="memory",P_=()=>({rx:Wn,ry:Ws,rz:Xs});function nf(n,t){zd(Pt,$e,P_,n),rh()>0&&(gh=t,Na[t]=!1)}function I_(n){let t=kd(n);return t.trabajo?(t.termino&&(Ir[gh]=new Map(Pt.map(e=>[e.id,{x:e.x,y:e.y,z:e.z,ph:e.ph}])),Na[gh]=!0),!0):!1}var L_=(n,t,e)=>n*n/(Wn*Wn)+t*t/(Ws*Ws)+e*e/(Xs*Xs)<=1;function sf(){for(let n=0;n<80;n++){let t=(Math.random()*2-1)*Wn,e=(Math.random()*2-1)*Ws,i=(Math.random()*2-1)*Xs;if(L_(t,e,i))return{x:t,y:e,z:i}}return{x:0,y:0,z:0}}var Pt=[],$e=[],Ir={memory:new Map,code:new Map},Na={memory:!1,code:!1},Ls=new Map,xh=new Set,li=0,Er=new Float32Array(0),Bs=new Float32Array(0),Ma=new Int8Array(0),En=!0,qi=!1,ye="memory",fi=Pd(document.getElementById("personas")),Lr=[],se=null,An=null,rf=null,br=null,da=0;function D_(n){let t=Ir.memory,e=n.neurons||[];Ah=Math.max(.85,Math.min(2.2,Math.cbrt((e.length||1)/240))),ef(),e.forEach(u=>{let p=_d(u);u._grupo=p.clave,u._grupoTipo=p.tipo});let s={};e.forEach(u=>s[u._grupo]=(s[u._grupo]||0)+1);let r=yd(Object.keys(s),s);Pr=new Map,vn=[];let o=new Map,a=.62,c=.32,l=Math.max(1,e.length/Math.max(r.length,1));r.forEach((u,p)=>{let b=C_(u,p);Pr.set(u,b),o.set(u,p);let y=p+.5,A=Math.acos(1-2*y/Math.max(r.length,1)),R=Math.PI*(1+Math.sqrt(5))*y;vn.push({name:u,color:b,count:s[u],ax:Math.cos(R)*Math.sin(A)*Wn*a,ay:Math.cos(A)*Ws*a,az:Math.sin(R)*Math.sin(A)*Xs*a,ar:Wn*c*Math.cbrt(s[u]/l)})});let f=new Set(t.keys());Pt=e.map(u=>{let p=t.get(u.id),b=p?{x:p.x,y:p.y,z:p.z}:sf(),y=Math.max(.9,Math.min(6,.9+Math.sqrt(Math.max(u.importance,0))*.72+Math.log(1+(u.heat||0))*.38)),A=Math.max(.1,Math.min(1,1-(u.recency_days||0)/45)),R=o.has(u._grupo)?vn[o.get(u._grupo)]:null;return{...u,x:b.x,y:b.y,z:b.z,vx:0,vy:0,vz:0,r:y,rec:A,col:$d(u._grupo),gx:R?R.ax:0,gy:R?R.ay:0,gz:R?R.az:0,gr:R?R.ar:0,ph:p&&p.ph!=null?p.ph:Math.random()*6.283,phx:Math.random()*6.283,phz:Math.random()*6.283,di:o.has(u._grupo)?o.get(u._grupo):-1,act:0,ak:0,adj:[],_new:!p}});let d=new Map(Pt.map((u,p)=>[u.id,p]));$e=(n.synapses||[]).filter(u=>d.has(u.source)&&d.has(u.target)).map(u=>{let p=tf(u.source+">"+u.target);return{...u,a:d.get(u.source),b:d.get(u.target),off:p}}),Pt.forEach(u=>{u.adj=[],u.deg=0});for(let u of $e){let p=.35+(u.confidence||0)*.55;Pt[u.a].adj.push({j:u.b,w:p}),Pt[u.b].adj.push({j:u.a,w:p}),Pt[u.a].deg++,Pt[u.b].deg++}if(Er=new Float32Array(Pt.length),Bs=new Float32Array(Pt.length),Ma=new Int8Array(Pt.length),Ls.size===0)li=0;else{let u=0;for(let p of Pt){let b=Ls.get(p.id);b?(p.heat>b.heat||b.rec!=null&&p.recency_days!=null&&p.recency_days<b.rec-4e-4)&&(p.act=1,p.ak=2,u++):p.age_days!=null&&p.age_days<.02&&(p.act=1,p.ak=1,u++)}for(let p of $e)!xh.has(p.source+"|"+p.target)&&Ls.has(p.source)&&Ls.has(p.target)&&(Pt[p.a].act=1,Pt[p.a].ak=3,Pt[p.b].act=1,Pt[p.b].ak=3,u++);u>0&&(li=Math.min(1,li+.5+u*.2),Mh=!0)}Ls=new Map(Pt.map(u=>[u.id,{heat:u.heat,rec:u.recency_days}])),xh=new Set($e.map(u=>u.source+"|"+u.target));let m=Pt.reduce((u,p)=>u+(p._new?1:0),0),x=m>0||Pt.length!==f.size;pf(n),Ir.memory=new Map(Pt.map(u=>[u.id,{x:u.x,y:u.y,z:u.z,ph:u.ph}]));let _=sh(Pt.length,m,Na.memory);_>0?(nf(_,"memory"),qi=!0):x&&(qi=!0)}function U_(n){let t=Ir.code,e=n.nodes||[];Ah=Math.max(.85,Math.min(2.2,Math.cbrt((e.length||1)/240))),ef();let s={};e.forEach(h=>s[h.module]=(s[h.module]||0)+1);let r=Object.keys(s).sort((h,m)=>s[m]-s[h]||h.localeCompare(m));Pr=new Map,vn=[];let o=new Map;r.forEach((h,m)=>{let x=ya[m%ya.length];Pr.set(h,x),o.set(h,m),vn.push({name:h,color:x,count:s[h]})});let a=new Set(t.keys());Pt=e.map(h=>{let m=t.get(h.key),x=m?{x:m.x,y:m.y,z:m.z}:sf(),_=Math.max(.9,Math.min(6,.9+Math.sqrt(Math.max(h.degree||0,0))*.62));return{id:h.key,topic:h.name||h.key,domain:h.module,mem_type:h.kind,heat:h.degree||0,path:h.path,line:h.line,gist:h.path?h.path+(h.line?":"+h.line:""):h.kind||"",_code:!0,_exp:void 0,x:x.x,y:x.y,z:x.z,vx:0,vy:0,vz:0,r:_,rec:1,col:$d(h.module),ph:m&&m.ph!=null?m.ph:Math.random()*6.283,phx:Math.random()*6.283,phz:Math.random()*6.283,di:o.has(h.module)?o.get(h.module):-1,act:0,ak:0,adj:[],_new:!m}});let c=new Map(Pt.map((h,m)=>[h.id,m]));$e=(n.edges||[]).filter(h=>c.has(h.source)&&c.has(h.target)).map(h=>{let m=tf(h.source+">"+h.target);return{...h,a:c.get(h.source),b:c.get(h.target),off:m}}),Pt.forEach(h=>{h.adj=[],h.deg=0});for(let h of $e){let m=.35+(h.confidence||0)*.55;Pt[h.a].adj.push({j:h.b,w:m}),Pt[h.b].adj.push({j:h.a,w:m}),Pt[h.a].deg++,Pt[h.b].deg++}Er=new Float32Array(Pt.length),Bs=new Float32Array(Pt.length),Ma=new Int8Array(Pt.length),Ls=new Map,xh=new Set,li=0;let l=Pt.reduce((h,m)=>h+(m._new?1:0),0),f=l>0||Pt.length!==a.size;Ir.code=new Map(Pt.map(h=>[h.id,{x:h.x,y:h.y,z:h.z,ph:h.ph}]));let d=sh(Pt.length,l,Na.code);d>0?(nf(d,"code"),qi=!0):f&&(qi=!0)}function N_(n){if(!hi)return;let t=Math.min(le,Pt.length);if(n>=1){for(let e=0;e<t;e++){let i=Pt[e];hi[e]=i.x,Gi[e]=i.y,Wi[e]=i.z}return}for(let e=0;e<t;e++){let i=Pt[e];hi[e]+=(i.x-hi[e])*n,Gi[e]+=(i.y-Gi[e])*n,Wi[e]+=(i.z-Wi[e])*n}}function O_(){let n=Pt.length;if(n){Er.fill(0),Bs.fill(0);for(let t=0;t<n;t++){let e=Pt[t].act;if(e<.012)continue;let i=Pt[t].ak,s=Pt[t].adj;for(let r=0;r<s.length;r++){let o=s[r].j,a=e*s[r].w;Er[o]+=a,a>Bs[o]&&(Bs[o]=a,Ma[o]=i)}}for(let t=0;t<n;t++){let e=Pt[t];e.act<.15&&Bs[t]>0&&(e.ak=Ma[t]),e.act=Math.min(1,e.act*.93+Er[t]*.11)}}}var F_=document.getElementById("brain"),we=new Po({canvas:F_,antialias:!0});we.setPixelRatio(Math.min(devicePixelRatio,2));we.info.autoReset=!1;var Zs=new Lo;Zs.fog=new Io(329485,.0016);var Tn=new Ne(58,innerWidth/innerHeight,1,6e3);Tn.position.set(0,20,340);var on=new $n;Zs.add(on);Zs.add(new Fo(2637919,.8));var of=new fr(16773340,1.15);of.position.set(-.5,.9,.7);Zs.add(of);var af=new fr(7314431,.4);af.position.set(.6,-.4,-.55);Zs.add(af);var cf=new Do(1,1),Eh=new Uo({color:16777215,roughness:.4,metalness:0,flatShading:!0});Eh.onBeforeCompile=n=>{n.fragmentShader=n.fragmentShader.replace("#include <opaque_fragment>",`float _fres=pow(1.0-max(dot(normalize(normal),normalize(vViewPosition)),0.0),2.4);
  outgoingLight+=diffuseColor.rgb*_fres*1.5;
#include <opaque_fragment>`)};var B_=new dr(1,1,1,6,1,!0),z_=["attribute vec3 aColor; attribute float aSpeed; attribute float aGlow; attribute float aBase;","varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;","void main(){ vUv=uv; vColor=aColor; vSpeed=aSpeed; vGlow=aGlow; vBase=aBase;","  gl_Position=projectionMatrix*modelViewMatrix*instanceMatrix*vec4(position,1.0); }"].join(`
`),k_=["precision highp float;","uniform float uTime;","varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;","float band(float y){ float p=fract(y); return smoothstep(0.0,0.06,p)*(1.0-smoothstep(0.06,0.36,p)); }","void main(){ float y=vUv.y-uTime*vSpeed; float pulse=band(y)+band(y+0.5); float i=vBase+pulse*vGlow; gl_FragColor=vec4(vColor*i,i); }"].join(`
`),lf=new dr(1,1,1,5,1,!0),hf=`
attribute vec3 aColor; attribute float aTaper; attribute float aGlow; attribute float aBase; attribute float aWarn;
varying vec3 vColor; varying float vGlow; varying float vBase; varying float vY; varying float vWarn;
void main(){ vColor=aColor; vGlow=aGlow; vBase=aBase; vWarn=aWarn; vY=position.y+0.5;
  vec3 p=position; p.xz*=mix(1.0,aTaper,vY);
  gl_Position=projectionMatrix*modelViewMatrix*instanceMatrix*vec4(p,1.0); }
`,uf=`
precision highp float;
varying vec3 vColor; varying float vGlow; varying float vBase; varying float vY; varying float vWarn;
void main(){ float i=vBase+vGlow*(1.0-0.35*vY);
  // EL FRENTE SATURA: un impulso fuerte quema hacia el blanco, uno debil apenas tine la rama.
  vec3 c=mix(vColor, vec3(1.0), clamp(vGlow*0.5, 0.0, 0.8));
  // Y si fallo, el nucleo se va a AMBAR ENTERO, sin dejar nada del color de la rama. Las dos
  // formas intermedias que probe no se leian, y las dos por la misma razon \u2014quedaba cian adentro
  // del nucleo\u2014: tenir el DESTINO de la saturacion deja el 20 % que el clamp no cubre, y tenir
  // con el ambar palido del HUD sobre un medio aditivo es casi blanco (ver impulsos.mjs).
  c=mix(c, vec3(${Ad.join(", ")}), vWarn);
  gl_FragColor=vec4(c*i,i); }
`,an=new yt;we.getDrawingBufferSize(an);var js=new Qo(we,new fe(an.x,an.y,{samples:4}));js.setSize(an.x,an.y);js.addPass(new $o(Zs,Tn));var H_=new Rs(new yt(an.x,an.y),.95,.7,.28);js.addPass(H_);js.addPass(new ea(an.x,an.y));var Zi=new Zo(Tn,we.domElement);Zi.rotateSpeed=2.4;Zi.zoomSpeed=1.3;Zi.panSpeed=.6;Zi.dynamicDampingFactor=.12;Zi.staticMoving=!1;var oe=null,le=0,hi,Gi,Wi,Vd,ba,Sa,wa,Ds,Us,Ns,Yi,oh,Os,Qe=null,zs=null,rn=null,ks=null,Hs=null,fa=null,_n=null,Tr=null,We=null,Fs=null,Cr=null,pa=null,ma=null,Aa=null,xn=[],Ea=null,Ta=null,Dr=null,Ur=null,qs=new Map,Gn=null,Rr=null,vh=Ed(),_h=!1,ga=null,xa=null,yh=0,Mh=!1,bh=-1,ui=new $t,ee=new Ot,Gd=new Ot,Ca=new D,Ra=new D,V_=new nn,Wd=!1;function G_(){oe&&(on.remove(oe),oe.geometry.dispose(),oe=null),Qe&&(on.remove(Qe),Qe.geometry.dispose(),Qe=null),zs&&(zs.dispose(),zs=null),_n&&(on.remove(_n),_n.geometry.dispose(),_n=null),We&&(on.remove(We),We.geometry.dispose(),We=null),Tr&&(Tr.dispose(),Tr=null),Gn&&(on.remove(Gn),Gn.geometry.dispose(),Gn=null),Rr&&(Rr.dispose(),Rr=null),rn=ks=Hs=fa=null,Fs=Cr=pa=ma=Aa=Ea=Ta=Dr=Ur=null,qs=new Map}var Pa=new ke,Vs=new D,Ia=new D,di=new D,df=new D(0,1,0);function W_(n){let t=new Map;for(let r of n){let o=r.persona||"";o&&(t.has(o)||t.set(o,[]),t.get(o).push({id:r.id,notas:r.notas}))}let e=vn.filter(r=>t.has(r.name)).map(r=>({persona:r.name,color:r.color,centro:[r.ax,r.ay,r.az],radio:r.ar,troncos:t.get(r.name).slice().sort((o,a)=>a.notas-o.notas)})),i=e.length?e.reduce((r,o)=>r+o.radio,0)/e.length:60,{troncos:s}=wd(e,{escala:Math.max(.6,i/34),topePorArbol:2200});return s}function ff(){if(!se)return;let n=md(se.terminales,An&&An.censo);se.actores=n.actores,se.sinDeclarar=n.sinDeclarar,se.censo=An,rf=gd(se.terminales,n.actores)}function pf(n){return se=Md(n),ff(),Lr=bd(se.terminales,se.actores),xn=W_(se.terminales),se}function X_(){if(ye==="code")return;let n=xn.reduce((i,s)=>i+s.segs.length,0);if(!n)return;Fs=new Float32Array(n*3),Cr=new Float32Array(n),pa=new Float32Array(n),ma=new Float32Array(n),Ea=new Float32Array(n),Ta=new Int32Array(n),Aa=new Float32Array(n),Dr=new Float32Array(xn.length),Ur=new Float32Array(xn.length),qs=new Map;let t=lf.clone();t.setAttribute("aColor",new ve(Fs,3)),t.setAttribute("aGlow",new ve(Cr,1)),t.setAttribute("aBase",new ve(pa,1)),t.setAttribute("aTaper",new ve(ma,1)),t.setAttribute("aWarn",new ve(Aa,1)),Tr=new te({uniforms:{},vertexShader:hf,fragmentShader:uf,transparent:!0,blending:ni,depthWrite:!1}),_n=new ii(t,Tr,n),_n.frustumCulled=!1;let e=0;xn.forEach((i,s)=>{ee.set(i.color||"#7f9cc9"),qs.set(i.id,s),Dr[s]=i.alcanceRama||1,Ur[s]=i.rSoma||1;for(let r of i.segs){Vs.set(r.a[0],r.a[1],r.a[2]),Ia.set(r.b[0],r.b[1],r.b[2]),di.subVectors(Ia,Vs);let o=di.length()||.001;Pa.setFromUnitVectors(df,di.normalize()),ui.compose(Ca.copy(Vs).addScaledVector(di,o*.5),Pa,Ra.set(r.w0,o,r.w0)),_n.setMatrixAt(e,ui),Fs[e*3]=ee.r,Fs[e*3+1]=ee.g,Fs[e*3+2]=ee.b,ma[e]=Math.max(.05,r.w1/r.w0),pa[e]=Math.max(.09,.62*Math.pow(.74,r.nivel)),Ea[e]=r.dist,Ta[e]=s,Cr[e]=0,e++}}),_n.instanceMatrix.needsUpdate=!0,on.add(_n),We=new ii(cf,Eh,xn.length),xn.forEach((i,s)=>{ui.compose(Ca.set(i.centro[0],i.centro[1],i.centro[2]),new ke,Ra.setScalar(i.rSoma)),We.setMatrixAt(s,ui),We.setColorAt(s,ee.set(i.color||"#7f9cc9"))}),_h=!0,We.instanceMatrix.needsUpdate=!0,We.instanceColor&&(We.instanceColor.needsUpdate=!0),on.add(We),q_()}function q_(){let n=se&&se.despachos||[],t=[];for(let c of n){let l=qs.get(c.de),f=qs.get(c.a);l===void 0||f===void 0||l===f||t.push({a:l,b:f,veces:Math.max(1,Number(c.veces)||1),cruza:xn[l].persona!==xn[f].persona})}if(!t.length)return;let e=new Float32Array(t.length*3),i=new Float32Array(t.length),s=new Float32Array(t.length),r=new Float32Array(t.length),o=new Float32Array(t.length),a=lf.clone();a.setAttribute("aColor",new ve(e,3)),a.setAttribute("aGlow",new ve(i,1)),a.setAttribute("aBase",new ve(s,1)),a.setAttribute("aTaper",new ve(r,1)),a.setAttribute("aWarn",new ve(o,1)),Rr=new te({uniforms:{},vertexShader:hf,fragmentShader:uf,transparent:!0,blending:ni,depthWrite:!1}),Gn=new ii(a,Rr,t.length),Gn.frustumCulled=!1,t.forEach((c,l)=>{let f=xn[c.a],d=xn[c.b];Vs.set(f.centro[0],f.centro[1],f.centro[2]),Ia.set(d.centro[0],d.centro[1],d.centro[2]),di.subVectors(Ia,Vs);let h=di.length()||.001;Pa.setFromUnitVectors(df,di.normalize());let m=.35+Math.log(1+c.veces)*.3;ui.compose(Ca.copy(Vs).addScaledVector(di,h*.5),Pa,Ra.set(m,h,m)),Gn.setMatrixAt(l,ui),ee.set(c.cruza?mh:ph),e[l*3]=ee.r,e[l*3+1]=ee.g,e[l*3+2]=ee.b,r[l]=.22,s[l]=c.cruza?.34:.2,i[l]=0,o[l]=0}),Gn.instanceMatrix.needsUpdate=!0,on.add(Gn)}function Y_(){if(G_(),le=Pt.length,!le)return;hi=new Float32Array(le),Gi=new Float32Array(le),Wi=new Float32Array(le),Vd=new Float32Array(le),ba=new Float32Array(le),Sa=new Float32Array(le),wa=new Float32Array(le),Ds=new Float32Array(le),Us=new Float32Array(le),Ns=new Float32Array(le),Yi=new Float32Array(le),oh=[],Os=Array.from({length:le},()=>[]),oe=new ii(cf,Eh,le),ga=new Uint8Array(le),yh=1,bh=-1;for(let t=0;t<le;t++){let e=Pt[t];hi[t]=e.x,Gi[t]=e.y,Wi[t]=e.z,Vd[t]=e.r,ba[t]=e.phx,Sa[t]=e.ph,wa[t]=e.phz,oh.push(new ke().setFromEuler(V_.set(Math.random()*6.283,Math.random()*6.283,Math.random()*6.283))),ui.compose(Ca.set(e.x,e.y,e.z),oh[t],Ra.set(e.r,e.r,e.r)),oe.setMatrixAt(t,ui),oe.setColorAt(t,ee.set(e.col))}oe.instanceMatrix.needsUpdate=!0,oe.instanceColor&&(oe.instanceColor.needsUpdate=!0),on.add(oe);let n=$e.length;if(n){rn=new Float32Array(n*3),ks=new Float32Array(n),Hs=new Float32Array(n),fa=new Float32Array(n),xa=new Uint8Array(n);let t=B_.clone();t.setAttribute("aColor",new ve(rn,3)),t.setAttribute("aSpeed",new ve(ks,1)),t.setAttribute("aGlow",new ve(Hs,1)),t.setAttribute("aBase",new ve(fa,1)),zs=new te({uniforms:{uTime:{value:0}},vertexShader:z_,fragmentShader:k_,transparent:!0,blending:ni,depthWrite:!1}),Qe=new ii(t,zs,n),Qe.frustumCulled=!1;for(let e=0;e<n;e++){let i=$e[e];Os[i.a].push(i.b),Os[i.b].push(i.a),i.__i=e,i.__r=.28+(i.confidence||0)*.5,ee.set(Qd(i)),rn[e*3]=ee.r,rn[e*3+1]=ee.g,rn[e*3+2]=ee.b,ks[e]=.42+(i.confidence||0)*.5,Hs[e]=.55,fa[e]=.06}on.add(Qe)}else for(let t of $e)Os[t.a].push(t.b),Os[t.b].push(t.a);if(X_(),!Wd&&le){let t=0;for(let e of Pt){let i=Math.hypot(e.x,e.y,e.z);i>t&&(t=i)}Tn.position.set(0,20,Math.max(240,t*2.7)),Wd=!0}qi=!1}var Ys=new zo,Xi=new yt(-9,-9),La=0,Da=0,_e=-1,va=-1,mf=new dn,Sr=new D,Xd=new D,sn=document.getElementById("tip");addEventListener("pointermove",n=>{Xi.x=n.clientX/innerWidth*2-1,Xi.y=-(n.clientY/innerHeight)*2+1,La=n.clientX,Da=n.clientY});we.domElement.addEventListener("pointerdown",n=>{if(!oe)return;Xi.x=n.clientX/innerWidth*2-1,Xi.y=-(n.clientY/innerHeight)*2+1,Ys.setFromCamera(Xi,Tn);let t=Ys.intersectObject(oe);if(t.length){_e=t[0].instanceId,va=n.pointerId;try{we.domElement.setPointerCapture(n.pointerId)}catch{}we.domElement.style.cursor="grabbing",Tn.getWorldDirection(Xd),mf.setFromNormalAndCoplanarPoint(Xd,t[0].point),Yi.fill(0);for(let e of Os[_e])Yi[e]=.55;n.stopImmediatePropagation(),n.preventDefault()}},!0);function Oa(){if(_e>=0){_e=-1,we.domElement.style.cursor="",Yi&&Yi.fill(0);try{va>=0&&we.domElement.releasePointerCapture(va)}catch{}va=-1}}we.domElement.addEventListener("pointerup",Oa,!0);we.domElement.addEventListener("pointercancel",Oa,!0);addEventListener("pointerup",Oa);addEventListener("blur",Oa);function Z_(){if(_e>=0||!oe){sn.classList.remove("on");return}Ys.setFromCamera(Xi,Tn);let n=Ys.intersectObject(oe);if(n.length){let t=Pt[n[0].instanceId];if(sn.querySelector(".tt").textContent=t.topic||t.domain,t._code){j_(t);let o=pe(t.gist||"(sin ruta)");Array.isArray(t._exp)&&t._exp.length&&(o+='<div class="why">'+t._exp.slice(0,3).map(c=>`<span>${pe(c.topic_key)}</span>`).join("")+"</div>"),sn.querySelector(".tg").innerHTML=o;let a=`<i>${pe(Hd(t))}</i><i>${pe(t.domain)}</i><i>centralidad ${t.heat}</i>`;Array.isArray(t._exp)&&(a+=t._exp.length?`<i style="color:var(--purple)">explicado \xD7${t._exp.length}</i>`:'<i style="opacity:.55">sin memorias</i>'),sn.querySelector(".tm").innerHTML=a}else sn.querySelector(".tg").textContent=t.gist||"(sin resumen)",sn.querySelector(".tm").innerHTML=`<i>${pe(Hd(t))}</i><i>${pe(t.domain)}</i><i>calor ${t.heat}</i>`;let e=sn.offsetWidth||220,i=sn.offsetHeight||70,s=La+16,r=Da+16;s+e>innerWidth-8&&(s=La-e-16),r+i>innerHeight-8&&(r=Da-i-16),sn.style.left=s+"px",sn.style.top=r+"px",sn.classList.add("on")}else sn.classList.remove("on")}async function j_(n){if(!(n._exp!==void 0||n._expLoading)){n._expLoading=!0;try{let t=await fetch("/api/explained?symbol="+encodeURIComponent(n.id),{cache:"no-store"});n._exp=t.ok?await t.json()||[]:[]}catch{n._exp=[]}finally{n._expLoading=!1}}}var la=2.4,_a=new URLSearchParams(location.search).has("stats"),Ar=null,ah=0,ch=0,ha=0,qd=0;_a&&(Ar=document.createElement("div"),Ar.style.cssText="position:fixed;left:50%;top:8px;transform:translateX(-50%);z-index:99;font:11px/1.5 ui-monospace,Menlo,monospace;color:#9fe8ff;background:rgba(6,10,18,.86);border:1px solid #23405c;border-radius:7px;padding:7px 12px;white-space:pre;pointer-events:none;letter-spacing:.02em;text-align:center",Ar.textContent="midiendo\u2026",document.body.appendChild(Ar));function gf(){if(requestAnimationFrame(gf),we.info.reset(),fi.activa()){fi.frame(En);return}let n=_a?performance.now():0;(qi||oe&&(le!==Pt.length||(Qe?Qe.count:0)!==$e.length))&&Y_();let t=I_(6);t&&N_(rh()>0?.16:1);let e=performance.now()/1e3;if(En&&(li*=.985,O_()),oe&&le){_e>=0&&(Ys.setFromCamera(Xi,Tn),Ys.ray.intersectPlane(mf,Sr)?on.worldToLocal(Sr):_e=-1);let s=0,r=0,o=0;if(_e>=0){let a=hi[_e]+Math.sin(e*.5+ba[_e])*la,c=Gi[_e]+Math.sin(e*.44+Sa[_e])*la,l=Wi[_e]+Math.sin(e*.57+wa[_e])*la;s=Sr.x-a,r=Sr.y-c,o=Sr.z-l,Ds[_e]=s,Us[_e]=r,Ns[_e]=o}if(En||_e>=0||yh>.002||Mh||t){let a=0,c=!1,l=!1,f=oe.instanceMatrix.array;for(let d=0;d<le;d++){let h=Pt[d],m=En?la:0,x=hi[d]+Math.sin(e*.5+ba[d])*m,_=Gi[d]+Math.sin(e*.44+Sa[d])*m,u=Wi[d]+Math.sin(e*.57+wa[d])*m;if(d!==_e)if(Yi[d]>0){let N=Yi[d];Ds[d]+=(s*N-Ds[d])*.14,Us[d]+=(r*N-Us[d])*.14,Ns[d]+=(o*N-Ns[d])*.14}else Ds[d]*=.945,Us[d]*=.945,Ns[d]*=.945;let p=Ds[d],b=Us[d],y=Ns[d],A=Math.abs(p)+Math.abs(b)+Math.abs(y);A>a&&(a=A);let R=x+p,T=_+b,E=u+y;h.x=R,h.y=T,h.z=E;let C=d*16;f[C+12]=R,f[C+13]=T,f[C+14]=E,h.act>.06?(c=!0,ee.set(h.col),Gd.set(fh[h.ak]||Jd),ee.lerp(Gd,Math.min(.85,h.act*.8)),oe.setColorAt(d,ee),ga[d]=1,l=!0):ga[d]&&(oe.setColorAt(d,ee.set(h.col)),ga[d]=0,l=!0)}if(oe.instanceMatrix.needsUpdate=!0,l&&oe.instanceColor&&(oe.instanceColor.needsUpdate=!0),Qe){zs.uniforms.uTime.value=e;let d=Qe.instanceMatrix.array,h=Math.abs(li-bh)>.002;h&&(bh=li);let m=!1;for(let x=0;x<$e.length;x++){let _=$e[x],u=Pt[_.a],p=Pt[_.b],b=p.x-u.x,y=p.y-u.y,A=p.z-u.z,R=Math.sqrt(b*b+y*y+A*A)||1,T=1/R;b*=T,y*=T,A*=T;let E,C,N;Math.abs(y)<.9?(E=A,C=0,N=-b):(E=0,C=-A,N=y);let g=Math.sqrt(E*E+C*C+N*N)||1,S=1/g;E*=S,C*=S,N*=S;let F=C*A-N*y,V=N*b-E*A,q=E*y-C*b,et=_.__r,B=x*16;d[B]=E*et,d[B+1]=C*et,d[B+2]=N*et,d[B+3]=0,d[B+4]=b*R,d[B+5]=y*R,d[B+6]=A*R,d[B+7]=0,d[B+8]=F*et,d[B+9]=V*et,d[B+10]=q*et,d[B+11]=0,d[B+12]=(u.x+p.x)*.5,d[B+13]=(u.y+p.y)*.5,d[B+14]=(u.z+p.z)*.5,d[B+15]=1;let it=Math.max(u.act,p.act),Z=u.act>=p.act?u.ak:p.ak;it>.06&&Z>0?(ee.set(fh[Z]),Hs[x]=.55+it*3.6,ks[x]=1+it*1.6,rn[x*3]=ee.r,rn[x*3+1]=ee.g,rn[x*3+2]=ee.b,xa[x]=1,m=!0):(xa[x]||h)&&(ee.set(Qd(_)),Hs[x]=.5+li*.35,ks[x]=.42+(_.confidence||0)*.5,rn[x*3]=ee.r,rn[x*3+1]=ee.g,rn[x*3+2]=ee.b,xa[x]=0,m=!0)}if(Qe.instanceMatrix.needsUpdate=!0,m){let x=Qe.geometry.attributes;x.aColor.needsUpdate=!0,x.aSpeed.needsUpdate=!0,x.aGlow.needsUpdate=!0}}yh=a,Mh=c}}if(_n&&Dr){let s=vh.vivos(e);if(s>0||_h){let r=vh.escribir(e,{glow:Cr,warn:Aa,dist:Ea,tronco:Ta,alcances:Dr}),o=_n.geometry.attributes;if(o.aGlow.needsUpdate=!0,o.aWarn.needsUpdate=!0,We){let a=We.instanceMatrix.array;for(let c=0;c<Ur.length;c++){let l=Ur[c]*(1+.9*r.flash[c]),f=c*16;a[f]=l,a[f+5]=l,a[f+10]=l}We.instanceMatrix.needsUpdate=!0}_h=s>0}}let i=_a?performance.now()-n:0;if(Zi.update(),Z_(),js.render(),_a){let s=performance.now();if(ah+=s-n,ch+=i,ha++,s-qd>500){let r=we.info.render,o=we.info.memory,a=ah/ha,c=ch/ha;Ar.textContent=Math.round(1e3/a)+" fps   frame "+a.toFixed(1)+" ms   \xB7   mis bucles "+c.toFixed(1)+" ms   \xB7   render+bloom "+(a-c).toFixed(1)+` ms
`+r.calls+" draw \xB7 "+(r.triangles/1e3).toFixed(0)+"k tris \xB7 dpr "+we.getPixelRatio()+" \xB7 "+o.geometries+" geo \xB7 "+o.textures+" tex",ah=ch=ha=0,qd=s}}}var Ht=n=>document.getElementById(n),pe=n=>(n==null?"":String(n)).replace(/[&<>"]/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[t]);function Sh(n){let t=ye==="code",e=n.brain||{neurons:[],synapses:[]},i=n.insights||{},s=i.observations||{},r=n.code||{nodes:[],edges:[]},o=t?(r.nodes||[]).length:(e.neurons||[]).length,a=t?r.total_nodes||0:e.total_neurons||0,c=t?r.truncated:e.truncated,l=t?(r.edges||[]).length:(e.synapses||[]).length,f=t?r.total_edges||0:e.total_synapses||0,h=(t?!!r.edges_truncated:!!e.synapses_truncated)?`${l}/${f}`:l;Ht("neuronCount").textContent=c?`${o}/${a}`:o,Ht("synCount").textContent=h,Ht("proj").textContent=n.project||"\u2014",Ht("ver").textContent=n.version||"";let m=(i.utilization||{}).active;Ht("kActive").textContent=t?a||o:m??(s.visible!=null?s.visible:s.active!=null?s.active:"\u2014"),Ht("kSyn").textContent=h;let x=(n.graph||{}).domains||[],_=vn.filter(N=>N.name!==xr&&N.name!==vr).length;Ht("kDomains").textContent=t?r.total_modules?r.truncated&&vn.length<r.total_modules?`${vn.length}/${r.total_modules}`:r.total_modules:vn.length:_;let u=vn.slice(0,10);ye==="personas"?Ch():Ht("domlegend").innerHTML=u.length?u.map(N=>`<div class="lg"><span class="sw" style="background:${N.color};color:${N.color}"></span>${pe(N.name)} <b>${N.count}</b></div>`).join(""):'<div class="empty">sin dominios</div>',ye==="personas"&&se&&(Ht("kActive").textContent=se.terminales.length,Ht("kSyn").textContent=se.despachos.reduce((N,g)=>N+g.veces,0),Ht("kDomains").textContent=Lr.filter(N=>N.persona!=="(servicios)").length);let p=(n.orchestration||{}).runs||[];Ht("kRuns").textContent=p.filter(N=>N.status==="running"||N.done<N.total).length;let b=n.health||{},y=(b.checks||[]).filter(N=>N.status&&N.status!=="ok").length;Ht("health").className="pill "+(b.status==="ok"?"ok":"warn"),Ht("healthTxt").textContent=b.status==="ok"?"sano":y?`${y} avisos`:"revisar",Ht("checks").innerHTML=(b.checks||[]).length?(b.checks||[]).map(N=>{let g=!N.status||N.status==="ok";return`<div class="chk"><span class="s" style="background:${g?"var(--green)":"var(--amber)"};box-shadow:0 0 7px ${g?"var(--green)":"var(--amber)"}"></span>${pe(N.code||N.name||"check")}</div>`}).join(""):'<div class="empty">sin checks</div>';let A=n.tokens||{},R=!!A.session_id,T=R&&A.status&&A.status!=="unbudgeted"&&A.budget;Ht("tokLabel").textContent=T?"Tokens de sesi\xF3n":R?"Tokens (sin techo)":"Tokens acumulados",Ht("tokVal").textContent=T?`${A.total||0} / ${A.budget}`:A.total||0;let E=Ht("tokBar");E.className="bar"+(T&&(A.status==="watch"||A.status==="over")?" watch":""),E.querySelector("i").style.width=Math.min(100,T?A.pct_used||0:A.total?8:0)+"%",Ht("runs").innerHTML=p.length?p.slice(0,6).map(N=>{let g=N.done||0,S=N.total||0,F=S?Math.round(g*100/S):0,V=N.status==="done"?"var(--green)":N.status==="failed"?"var(--red)":"var(--amber)";return`<div class="run"><span class="s" style="width:6px;height:6px;border-radius:50%;background:${V};box-shadow:0 0 7px ${V}"></span><span class="rl">${pe(N.workflow_id||N.run_id)}</span><span class="rp">${g}/${S}\xB7${F}%</span></div>`}).join(""):'<div class="empty">sin runs activos</div>';let C=n.recent||[];Ht("recent").innerHTML=C.length?C.slice(0,6).map(N=>`<div class="item"><span class="tk">${pe(N.topic_key)}</span><span class="gg">${pe(N.gist)}</span></div>`).join(""):'<div class="empty">memoria en formaci\xF3n</div>'}var Se={nuevos:[],vistos:new Set,sondeos:[],enlace:{estado:"conectando"}},xf=40,K_=600;function J_(n){let t=(n.origen||"central")+"|"+(n.seq||0)+"|"+(n.at||"");return Se.vistos.has(t)?!0:(Se.vistos.add(t),Se.vistos.size>K_&&(Se.vistos=new Set([...Se.vistos].slice(-xf*2))),!1)}var Yd=0;function Q_(){let n=performance.now();n-Yd<2e3||(Yd=n,setTimeout(()=>{Th().catch(()=>{})},350))}function Zd(n){if(!(!n||J_(n))){if(n.kind==="sondeo"){Se.sondeos.push(Date.now());return}n.perdidos>0&&Se.nuevos.push({hueco:n.perdidos}),Se.nuevos.push(n),n.tool==="musubi_codegraph_push"&&(Te.code=null),Q_()}}function vf(n){let t=Date.parse(n);if(!isFinite(t))return"";let e=Math.max(0,Math.round((Date.now()-t)/1e3));if(e<60)return e+"s";let i=Math.round(e/60);return i<60?i+"m":Math.round(i/60)+"h"}var $_=n=>String(n||"").replace(/^musubi_/,"");function ty(n){let t=document.createElement("div");if(n.hueco)return t.className="ev hueco",t.innerHTML=`<span class="s"></span><span class="t">se perdieron ${n.hueco} eventos</span>`,t;let e=n.origen==="local";return t.className="ev"+(e?" loc":"")+(n.outcome&&n.outcome!=="ok"?" err":""),t.dataset.at=n.at||"",t.innerHTML=`<span class="s"></span><span class="t">${pe($_(n.tool))}</span>`+(e?'<span class="q aqui">esta maquina</span>':n.principal?`<span class="q">${pe(n.principal)}</span>`:"")+`<span class="d">${vf(n.at)}</span>`,t}function ua(){let n=Ht("vivo");if(n){if(Se.nuevos.length){let t=n.querySelector(".empty");t&&t.remove();for(let e of Se.nuevos)n.insertBefore(ty(e),n.firstChild);for(Se.nuevos.length=0;n.children.length>xf;)n.removeChild(n.lastChild)}n.children.length||(n.innerHTML='<div class="empty">sin trabajo todavia</div>'),ey()}}function ey(){let n=Ht("enlace");if(!n)return;let t=Se.enlace,e="enl"+(t.estado==="conectado"?" ok":t.estado==="conectando"?"":" mal"),i=t.estado==="conectado"?t.destino||"enlazado":t.estado==="conectando"?"conectando\u2026":t.estado==="apagado"?"apagado":"sin enlace";n.className!==e&&(n.className=e),n.textContent!==i&&(n.textContent=i);let s=t.detalle||"";n.title!==s&&(n.title=s)}function ny(){let n=Ht("vivo");if(n)for(let s of n.children){let r=s.dataset&&s.dataset.at;if(!r)continue;let o=s.lastElementChild;if(!o||!o.classList.contains("d"))continue;let a=vf(r);o.textContent!==a&&(o.textContent=a)}let t=Date.now()-6e4;Se.sondeos=Se.sondeos.filter(s=>s>t);let e=Ht("latido"),i=Ht("latidoTxt");if(e&&i){let s=Se.sondeos.length,r=s>0?`sondeo \xB7 ${s}/min`:"sin sondeo";e.classList.toggle("vive",s>0),i.textContent!==r&&(i.textContent=r)}}setInterval(ny,1e3);function iy(n){if(!n||!n.tool)return;String(n.principal||"").trim()||da++;let t=xd(n,rf),e=vd(n),i=t?{terminal:t.terminal,exacta:t.exacta,capa:e.capa,falla:e.falla,ms:e.ms}:{terminal:"",capa:e.capa,falla:e.falla,ms:e.ms};fi.activa()&&fi.pulsar(i);let s=t?qs.get(t.terminal):void 0;vh.nacer(s===void 0?-1:s,i,performance.now()/1e3)}function sy(){let n;try{n=new EventSource("/api/stream")}catch{return}n.addEventListener("backlog",t=>{try{(JSON.parse(t.data)||[]).forEach(Zd)}catch{}ua()}),n.addEventListener("uso",t=>{try{let e=JSON.parse(t.data);Zd(e),iy(e)}catch{}ua()}),n.addEventListener("enlace",t=>{try{Se.enlace=JSON.parse(t.data),ua()}catch{}}),n.onerror=()=>{Se.enlace.estado!=="apagado"&&(Se.enlace={estado:"caido",detalle:"se corto el stream del panel"},ua())}}function _f(){let n=innerWidth,t=innerHeight;we.setSize(n,t),Tn.aspect=n/t,Tn.updateProjectionMatrix(),we.getDrawingBufferSize(an),js.setSize(an.x,an.y),Zi.handleResize()}addEventListener("resize",_f);_f();var Te={memory:null,code:null},Gs=null,lh=null,wr={};async function Ua(n){return wr[n]||(wr[n]=(async()=>{try{let t=await fetch("/api/graph?lens="+n,{cache:"no-store"});if(!t.ok)throw 0;Te[n]=await t.json()}finally{wr[n]=null}})()),wr[n]}async function ry(){return br||(br=(async()=>{try{let n=await fetch("/api/actores",{cache:"no-store"});if(!n.ok)throw 0;let t=await n.json();An={estado:t.estado,detalle:t.detalle||"",destino:t.destino||"",censo:t.censo||null}}catch{An={estado:"caido",detalle:"el panel no pudo pedir el censo",censo:null}}finally{br=null}})(),br)}function oy(n,t){if(!n)return!1;if(t.touched&&t.touched.length){let e=new Map(n.neurons.map((i,s)=>[i.id,s]));for(let i of t.touched){let s=e.get(i.id);s!=null&&(n.neurons[s].heat=i.heat,n.neurons[s].recency_days=i.recency_days)}}if(t.new_neurons&&t.new_neurons.length){let e=new Set(n.neurons.map(i=>i.id));for(let i of t.new_neurons)e.has(i.id)||(n.neurons.push(i),e.add(i.id))}if(t.new_synapses&&t.new_synapses.length){let e=new Set(n.synapses.map(i=>i.source+"|"+i.target));for(let i of t.new_synapses){let s=i.source+"|"+i.target;e.has(s)||(n.synapses.push(i),e.add(s))}}return n.total_neurons=t.counts.neurons,n.total_synapses=t.counts.synapses,n.truncated=n.neurons.length<n.total_neurons,n.synapses_truncated=n.synapses.length<n.total_synapses,n.neurons.length===t.counts.neurons}function wh(){let n=Gs||{};return{brain:Te.memory||{neurons:[],synapses:[]},code:Te.code||{nodes:[],edges:[]},insights:n.insights||{},graph:{domains:n.domains||[],total_observations:(n.counts||{}).neurons||0},health:n.health||{},tokens:n.tokens||{},recent:n.recent||[],orchestration:n.orchestration||{},project:n.project,version:n.version}}var hh=!1;async function Th(){if(!hh){hh=!0;try{let n=await fetch("/api/pulse"+(lh?"?since="+encodeURIComponent(lh):""),{cache:"no-store"});if(!n.ok)throw 0;Gs=await n.json(),ye==="code"?Te.code||await Ua("code"):(!Te.memory||!oy(Te.memory,Gs))&&await Ua("memory"),lh=Gs.now,Nr(),Sh(wh()),Ht("liveTxt").textContent="en vivo"}catch{Ht("liveTxt").textContent="reconectando"}finally{hh=!1}}}var uh=null;function Nr(){if(ye==="personas"){if(!Te.memory)return;let n=pf(Te.memory);fi.cargar(n,Lr),Ch();return}if(ye==="code"){if(!Te.code||uh===Te.code)return;U_(Te.code),uh=Te.code;return}Te.memory&&(D_(Te.memory),uh=Te.memory)}function yf(n){En=n;let t=Ht("motionBtn");t&&(t.textContent=En?"\u275A\u275A pausar":"\u25B6 reanudar",t.classList.toggle("paused",!En),t.setAttribute("aria-pressed",String(!En)))}Ht("motionBtn").addEventListener("click",()=>yf(!En));yf(En);function Mf(n){ye=n;let t=Ht("lensBtn");qi=!0;let e=document.getElementById("brain"),i=document.getElementById("personas"),s=ye==="personas";if(e&&(e.hidden=s),i&&(i.hidden=!s),fi.activar(s),t&&(t.textContent=s?"\u25C9 personas":ye==="code"?"\u25C9 c\xF3digo":"\u25C9 memoria",t.classList.toggle("code",ye==="code"),t.classList.toggle("personas",s),t.setAttribute("aria-pressed",String(ye!=="memory"))),bf(),ye==="code"&&!Te.code){Ua("code").then(()=>{Nr(),Gs&&Sh(wh())}).catch(()=>{});return}Nr(),Gs&&Sh(wh())}function bf(){let n=ye==="code",t=ye==="personas",e=(r,o)=>{let a=Ht(r);a&&(a.textContent=o)};if(e("lblNodes",n?"nodos":"neuronas"),e("lblEdges",n?"aristas":"sinapsis"),e("domTitle",t?"Personas":n?"M\xF3dulos":"Personas"),e("lblActive",t?"Terminales":n?"Nodos":"Memorias activas"),e("lblSyn",t?"Despachos":n?"Aristas":"Sinapsis"),e("lblDomains",t?"Personas":n?"M\xF3dulos":"Personas"),t){let r=Ht("actlegend");r&&(r.innerHTML=`<div class="lg"><span class="sw" style="background:rgba(255,255,255,.30);color:rgba(255,255,255,.30)"></span>sondeo</div><div class="lg"><span class="sw" style="background:#fff;color:#fff"></span>trabajo real</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>fall\xF3</div><div class="lg"><span class="sw" style="background:${ph};color:${ph}"></span>despacho</div><div class="lg"><span class="sw" style="background:${mh};color:${mh}"></span>cruza personas</div>`);let o=Ht("howto");o&&(o.innerHTML="<span><b>\xB7</b> cada neurona es una <b>terminal</b>; sus <b>dendritas</b>, cu\xE1nto escribi\xF3</span><span><b>\xB7</b> el <b>impulso</b> que recorre un \xE1rbol es <b>UNA llamada real a una tool</b>, en el momento en que ocurre. <b>Sin evento no hay luz</b>: si el cerebro est\xE1 quieto, esto est\xE1 quieto</span><span><b>\xB7</b> el <b>grosor</b> del impulso es lo que tard\xF3 esa llamada; el <b>\xE1mbar</b>, que fall\xF3</span><span><b>\xB7</b> la neurona <b>late</b> seg\xFAn cu\xE1nto se <b>recupera</b> lo que escribi\xF3 \u2014 la que nadie consulta se queda quieta</span><span><b>\xB7</b> el <b>color</b> agrupa por <b>persona</b> \xB7 la persona sale de qui\xE9n <b>firma</b>, no de qui\xE9n menciona</span>"),jd('<span><b>arrastr\xE1</b> gira</span><span class="sep">\xB7</span><span><b>rueda</b> acerca 40\xD7</span><span class="sep">\xB7</span><span><b>shift+arrastr\xE1</b> desplaza</span><span class="sep">\xB7</span><span><b>hover</b> detalla</span><span class="sep">\xB7</span><span><b>doble click</b> entra</span>'),Ch();return}jd('<span><b>arrastr\xE1</b> para rotar</span><span class="sep">\xB7</span><span><b>rueda</b> para acercar</span><span class="sep">\xB7</span><span><b>hover</b> revela el detalle</span>');let i=Ht("actlegend");i&&(i.innerHTML=n?`<div class="lg"><span class="sw" style="background:${Vi.CALLS};color:${Vi.CALLS}"></span>llama</div><div class="lg"><span class="sw" style="background:${Vi.IMPORTS};color:${Vi.IMPORTS}"></span>importa</div><div class="lg"><span class="sw" style="background:${Vi.CONTAINS};color:${Vi.CONTAINS}"></span>contiene</div>`:'<div class="lg"><span class="sw" style="background:#7f9cc9;color:#7f9cc9"></span>reposo</div><div class="lg"><span class="sw" style="background:#43e08b;color:#43e08b"></span>escribir</div><div class="lg"><span class="sw" style="background:#31c9ff;color:#31c9ff"></span>recordar</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>relacionar</div><div class="lg" style="opacity:.5;margin-left:6px">impulso:</div><div class="lg"><span class="sw" style="background:rgba(255,255,255,.32);color:rgba(255,255,255,.32)"></span>sondeo</div><div class="lg"><span class="sw" style="background:#fff;color:#fff"></span>trabajo real</div><div class="lg"><span class="sw" style="background:#ff9905;color:#ff9905"></span>fall\xF3</div>');let s=Ht("howto");s&&(s.innerHTML=n?"<span><b>\xB7</b> cada punto es un <b>s\xEDmbolo</b> (funci\xF3n, tipo, archivo)</span><span><b>\xB7</b> las l\xEDneas son <b>llamadas / imports</b>; el color agrupa por <b>m\xF3dulo</b></span><span><b>\xB7</b> el <b>tama\xF1o</b> = centralidad \xB7 <b>hover</b> muestra qu\xE9 memorias lo explican</span>":"<span><b>\xB7</b> cada punto es una <b>memoria</b></span><span><b>\xB7</b> las l\xEDneas, <b>relaciones</b>; la luz que viaja = <b>recuerdo activ\xE1ndose</b></span><span><b>\xB7</b> el <b>racimo</b> y el <b>color</b> agrupan por <b>qui\xE9n la escribi\xF3</b> \xB7 gris = no es de una persona</span><span><b>\xB7</b> la <b>neurona ramificada</b> de cada racimo es una <b>terminal</b>; sus dendritas, cu\xE1nto escribi\xF3</span><span><b>\xB7</b> el <b>impulso</b> que la recorre es <b>UNA llamada real a una tool</b>, en el momento en que ocurre. <b>Sin evento no hay luz</b>: si el cerebro est\xE1 quieto, esto est\xE1 quieto</span>")}fi.onFoco((n,t,e,i)=>{let s=document.getElementById("tip");if(!s)return;if(!n){s.classList.remove("on");return}let r=t&&t.despachos||[],o=(u,p)=>u.length?u.sort((b,y)=>y.veces-b.veces).slice(0,4).map(b=>`${pe((p==="sale"?b.a:b.de).toLowerCase())} <b>${b.veces}</b>`).join(" \xB7 "):"\u2014",a=r.filter(u=>u.de===n.id),c=r.filter(u=>u.a===n.id),l=u=>(u||0).toLocaleString("es");if(s.querySelector(".tt").textContent=n.id.toLowerCase(),n.tipo==="actor"){let u=n.llamadas||{};s.querySelector(".tg").innerHTML=n.exacta===!1?`credencial de <b>${pe(n.persona)}</b> \xB7 por convenci\xF3n del nombre, sin declarar`:`servicio \xB7 <b>due\xF1o no declarado</b>${u.proyecto?` \xB7 proyecto ${pe(u.proyecto)}`:""}`,s.querySelector(".tm").innerHTML=`<i>${l(u.calls)} llamadas</i><i>${l(u.trabajo)} trabajo \xB7 ${l(u.sondeo)} sondeo</i><i>${u.tools||0} tools distintas</i>`}else{s.querySelector(".tg").innerHTML=`de <b>${pe(n.persona||"sin autor")}</b> \xB7 ${n.notas} notas la nombran \xB7 ${n.firmas} las firma`;let u=n.llamadas;s.querySelector(".tm").innerHTML=`<i>calor ${n.calor}</i>${u?`<i>${l(u.calls)} llamadas \xB7 ${l(u.trabajo)} trabajo</i>`:""}<i>escribe a: ${o(a,"sale")}</i><i>recibe de: ${o(c,"entra")}</i>`}let f=s.offsetWidth||240,d=s.offsetHeight||80,h=typeof e=="number"?e:La,m=typeof i=="number"?i:Da,x=h+16,_=m+16;x+f>innerWidth-8&&(x=h-f-16),_+d>innerHeight-8&&(_=m-d-16),s.style.left=x+"px",s.style.top=_+"px",s.classList.add("on")});function jd(n){let t=Ht("pistas");t&&(t.innerHTML=n)}function Ch(){let n=Ht("domlegend");if(!n)return;if(!Lr.length){n.innerHTML='<div class="empty">sin terminales firmadas</div>';return}let t=Lr.map((c,l)=>{let f=Rd(l),d=c.persona==="(servicios)"?`<b>${c.llamadas}</b> llamadas`:`<b>${c.notas}</b>`;return`<div class="lg"><span class="sw" style="background:${f};color:${f}"></span>${pe(c.persona)} ${d}</div>`});An&&An.estado&&An.estado!=="vivo"&&t.push(`<div class="lg" style="opacity:.7">censo de actores ${pe(An.estado)} \xB7 ${pe(An.detalle||"")}</div>`);let e=se&&se.actores?se.actores:[];if(e.length){let c=e.reduce((l,f)=>l+f.calls,0);t.push(`<div class="lg" style="opacity:.55">${e.length} actores \u25EF \xB7 ${c.toLocaleString("es")} llamadas</div>`)}let i=se&&se.sinDeclarar||0;i&&t.push(`<div class="lg" style="opacity:.55">${i} sin due\xF1o declarado \xB7 van a servicios</div>`);let s=se?se.despachos.length:0;s&&t.push(`<div class="lg" style="opacity:.55">${s} pares se escriben</div>`);let r=se?se.sinAutor:0;r&&t.push(`<div class="lg" style="opacity:.55">${r} notas sin autor \xB7 no se reparten</div>`);let o=fi.cuentaPulsos();da&&t.push(`<div class="lg" style="opacity:.55">${da} eventos de esta m\xE1quina \xB7 sin credencial, no se atribuyen</div>`);let a=Math.max(0,o.sinNeurona-da);a&&t.push(`<div class="lg" style="opacity:.55">${a} eventos sin neurona \xB7 falta declarar su due\xF1o</div>`),n.innerHTML=t.join("")}var Kd=Ht("lensBtn");Kd&&Kd.addEventListener("click",()=>Mf(ye==="memory"?"code":ye==="code"?"personas":"memory"));bf();var dh=new URLSearchParams(location.search).get("lens");(dh==="code"||dh==="personas")&&Mf(dh);Ua(ye==="code"?"code":"memory").then(()=>{Nr()}).catch(()=>{});ry().then(()=>{ff(),ye==="personas"&&Nr()}).catch(()=>{});Th();setInterval(Th,5e3);sy();requestAnimationFrame(gf);})();
